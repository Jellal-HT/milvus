// Copyright (C) 2019-2020 Zilliz. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License
// is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
// or implied. See the License for the specific language governing permissions and limitations under the License.
package querycoord

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	etcdkv "github.com/milvus-io/milvus/internal/kv/etcd"
	"github.com/milvus-io/milvus/internal/log"
	"github.com/milvus-io/milvus/internal/proto/commonpb"
	"github.com/milvus-io/milvus/internal/proto/querypb"
)

type testTask struct {
	baseTask
	baseMsg *commonpb.MsgBase
	cluster Cluster
	meta    Meta
	nodeID  int64
}

func (tt *testTask) msgBase() *commonpb.MsgBase {
	return tt.baseMsg
}

func (tt *testTask) marshal() ([]byte, error) {
	return []byte{}, nil
}

func (tt *testTask) msgType() commonpb.MsgType {
	return tt.baseMsg.MsgType
}

func (tt *testTask) timestamp() Timestamp {
	return tt.baseMsg.Timestamp
}

func (tt *testTask) preExecute(ctx context.Context) error {
	tt.setResultInfo(nil)
	log.Debug("test task preExecute...")
	return nil
}

func (tt *testTask) execute(ctx context.Context) error {
	log.Debug("test task execute...")

	switch tt.baseMsg.MsgType {
	case commonpb.MsgType_LoadSegments:
		childTask := &loadSegmentTask{
			baseTask: &baseTask{
				ctx:              tt.ctx,
				Condition:        NewTaskCondition(tt.ctx),
				triggerCondition: tt.triggerCondition,
			},
			LoadSegmentsRequest: &querypb.LoadSegmentsRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_LoadSegments,
				},
				NodeID: tt.nodeID,
			},
			meta:           tt.meta,
			cluster:        tt.cluster,
			excludeNodeIDs: []int64{},
		}
		tt.addChildTask(childTask)
	case commonpb.MsgType_WatchDmChannels:
		childTask := &watchDmChannelTask{
			baseTask: &baseTask{
				ctx:              tt.ctx,
				Condition:        NewTaskCondition(tt.ctx),
				triggerCondition: tt.triggerCondition,
			},
			WatchDmChannelsRequest: &querypb.WatchDmChannelsRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_WatchDmChannels,
				},
				NodeID: tt.nodeID,
			},
			cluster:        tt.cluster,
			meta:           tt.meta,
			excludeNodeIDs: []int64{},
		}
		tt.addChildTask(childTask)
	case commonpb.MsgType_WatchQueryChannels:
		childTask := &watchQueryChannelTask{
			baseTask: &baseTask{
				ctx:              tt.ctx,
				Condition:        NewTaskCondition(tt.ctx),
				triggerCondition: tt.triggerCondition,
			},
			AddQueryChannelRequest: &querypb.AddQueryChannelRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_WatchQueryChannels,
				},
				NodeID: tt.nodeID,
			},
			cluster: tt.cluster,
		}
		tt.addChildTask(childTask)
	}

	return nil
}

func (tt *testTask) postExecute(ctx context.Context) error {
	log.Debug("test task postExecute...")
	return nil
}

func TestWatchQueryChannel_ClearEtcdInfoAfterAssignedNodeDown(t *testing.T) {
	refreshParams()
	baseCtx := context.Background()
	queryCoord, err := startQueryCoord(baseCtx)
	assert.Nil(t, err)
	activeTaskIDKeys, _, err := queryCoord.scheduler.client.LoadWithPrefix(activeTaskPrefix)
	assert.Nil(t, err)
	queryNode, err := startQueryNodeServer(baseCtx)
	assert.Nil(t, err)
	queryNode.addQueryChannels = returnFailedResult

	nodeID := queryNode.queryNodeID
	waitQueryNodeOnline(queryCoord.cluster, nodeID)
	testTask := &testTask{
		baseTask: baseTask{
			ctx:              baseCtx,
			Condition:        NewTaskCondition(baseCtx),
			triggerCondition: querypb.TriggerCondition_grpcRequest,
		},
		baseMsg: &commonpb.MsgBase{
			MsgType: commonpb.MsgType_WatchQueryChannels,
		},
		cluster: queryCoord.cluster,
		meta:    queryCoord.meta,
		nodeID:  nodeID,
	}
	queryCoord.scheduler.Enqueue(testTask)

	queryNode.stop()
	err = removeNodeSession(queryNode.queryNodeID)
	assert.Nil(t, err)
	for {
		newActiveTaskIDKeys, _, err := queryCoord.scheduler.client.LoadWithPrefix(activeTaskPrefix)
		assert.Nil(t, err)
		if len(newActiveTaskIDKeys) == len(activeTaskIDKeys) {
			break
		}
	}
	queryCoord.Stop()
	err = removeAllSession()
	assert.Nil(t, err)
}

func TestUnMarshalTask(t *testing.T) {
	refreshParams()
	kv, err := etcdkv.NewEtcdKV(Params.EtcdEndpoints, Params.MetaRootPath)
	assert.Nil(t, err)
	baseCtx, cancel := context.WithCancel(context.Background())
	taskScheduler := &TaskScheduler{
		ctx:    baseCtx,
		cancel: cancel,
	}

	t.Run("Test loadCollectionTask", func(t *testing.T) {
		loadTask := &loadCollectionTask{
			LoadCollectionRequest: &querypb.LoadCollectionRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_LoadCollection,
				},
			},
		}
		blobs, err := loadTask.marshal()
		assert.Nil(t, err)
		err = kv.Save("testMarshalLoadCollection", string(blobs))
		assert.Nil(t, err)
		defer kv.RemoveWithPrefix("testMarshalLoadCollection")
		value, err := kv.Load("testMarshalLoadCollection")
		assert.Nil(t, err)

		task, err := taskScheduler.unmarshalTask(1000, value)
		assert.Nil(t, err)
		assert.Equal(t, task.msgType(), commonpb.MsgType_LoadCollection)
	})

	t.Run("Test LoadPartitionsTask", func(t *testing.T) {
		loadTask := &loadPartitionTask{
			LoadPartitionsRequest: &querypb.LoadPartitionsRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_LoadPartitions,
				},
			},
		}
		blobs, err := loadTask.marshal()
		assert.Nil(t, err)
		err = kv.Save("testMarshalLoadPartition", string(blobs))
		assert.Nil(t, err)
		defer kv.RemoveWithPrefix("testMarshalLoadPartition")
		value, err := kv.Load("testMarshalLoadPartition")
		assert.Nil(t, err)

		task, err := taskScheduler.unmarshalTask(1001, value)
		assert.Nil(t, err)
		assert.Equal(t, task.msgType(), commonpb.MsgType_LoadPartitions)
	})

	t.Run("Test releaseCollectionTask", func(t *testing.T) {
		releaseTask := &releaseCollectionTask{
			ReleaseCollectionRequest: &querypb.ReleaseCollectionRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_ReleaseCollection,
				},
			},
		}
		blobs, err := releaseTask.marshal()
		assert.Nil(t, err)
		err = kv.Save("testMarshalReleaseCollection", string(blobs))
		assert.Nil(t, err)
		defer kv.RemoveWithPrefix("testMarshalReleaseCollection")
		value, err := kv.Load("testMarshalReleaseCollection")
		assert.Nil(t, err)

		task, err := taskScheduler.unmarshalTask(1002, value)
		assert.Nil(t, err)
		assert.Equal(t, task.msgType(), commonpb.MsgType_ReleaseCollection)
	})

	t.Run("Test releasePartitionTask", func(t *testing.T) {
		releaseTask := &releasePartitionTask{
			ReleasePartitionsRequest: &querypb.ReleasePartitionsRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_ReleasePartitions,
				},
			},
		}
		blobs, err := releaseTask.marshal()
		assert.Nil(t, err)
		err = kv.Save("testMarshalReleasePartition", string(blobs))
		assert.Nil(t, err)
		defer kv.RemoveWithPrefix("testMarshalReleasePartition")
		value, err := kv.Load("testMarshalReleasePartition")
		assert.Nil(t, err)

		task, err := taskScheduler.unmarshalTask(1003, value)
		assert.Nil(t, err)
		assert.Equal(t, task.msgType(), commonpb.MsgType_ReleasePartitions)
	})

	t.Run("Test loadSegmentTask", func(t *testing.T) {
		loadTask := &loadSegmentTask{
			LoadSegmentsRequest: &querypb.LoadSegmentsRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_LoadSegments,
				},
			},
		}
		blobs, err := loadTask.marshal()
		assert.Nil(t, err)
		err = kv.Save("testMarshalLoadSegment", string(blobs))
		assert.Nil(t, err)
		defer kv.RemoveWithPrefix("testMarshalLoadSegment")
		value, err := kv.Load("testMarshalLoadSegment")
		assert.Nil(t, err)

		task, err := taskScheduler.unmarshalTask(1004, value)
		assert.Nil(t, err)
		assert.Equal(t, task.msgType(), commonpb.MsgType_LoadSegments)
	})

	t.Run("Test releaseSegmentTask", func(t *testing.T) {
		releaseTask := &releaseSegmentTask{
			ReleaseSegmentsRequest: &querypb.ReleaseSegmentsRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_ReleaseSegments,
				},
			},
		}
		blobs, err := releaseTask.marshal()
		assert.Nil(t, err)
		err = kv.Save("testMarshalReleaseSegment", string(blobs))
		assert.Nil(t, err)
		defer kv.RemoveWithPrefix("testMarshalReleaseSegment")
		value, err := kv.Load("testMarshalReleaseSegment")
		assert.Nil(t, err)

		task, err := taskScheduler.unmarshalTask(1005, value)
		assert.Nil(t, err)
		assert.Equal(t, task.msgType(), commonpb.MsgType_ReleaseSegments)
	})

	t.Run("Test watchDmChannelTask", func(t *testing.T) {
		watchTask := &watchDmChannelTask{
			WatchDmChannelsRequest: &querypb.WatchDmChannelsRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_WatchDmChannels,
				},
			},
		}
		blobs, err := watchTask.marshal()
		assert.Nil(t, err)
		err = kv.Save("testMarshalWatchDmChannel", string(blobs))
		assert.Nil(t, err)
		defer kv.RemoveWithPrefix("testMarshalWatchDmChannel")
		value, err := kv.Load("testMarshalWatchDmChannel")
		assert.Nil(t, err)

		task, err := taskScheduler.unmarshalTask(1006, value)
		assert.Nil(t, err)
		assert.Equal(t, task.msgType(), commonpb.MsgType_WatchDmChannels)
	})

	t.Run("Test watchQueryChannelTask", func(t *testing.T) {
		watchTask := &watchQueryChannelTask{
			AddQueryChannelRequest: &querypb.AddQueryChannelRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_WatchQueryChannels,
				},
			},
		}
		blobs, err := watchTask.marshal()
		assert.Nil(t, err)
		err = kv.Save("testMarshalWatchQueryChannel", string(blobs))
		assert.Nil(t, err)
		defer kv.RemoveWithPrefix("testMarshalWatchQueryChannel")
		value, err := kv.Load("testMarshalWatchQueryChannel")
		assert.Nil(t, err)

		task, err := taskScheduler.unmarshalTask(1007, value)
		assert.Nil(t, err)
		assert.Equal(t, task.msgType(), commonpb.MsgType_WatchQueryChannels)
	})

	t.Run("Test loadBalanceTask", func(t *testing.T) {
		loadBalanceTask := &loadBalanceTask{
			LoadBalanceRequest: &querypb.LoadBalanceRequest{
				Base: &commonpb.MsgBase{
					MsgType: commonpb.MsgType_LoadBalanceSegments,
				},
			},
		}

		blobs, err := loadBalanceTask.marshal()
		assert.Nil(t, err)
		err = kv.Save("testMarshalLoadBalanceTask", string(blobs))
		assert.Nil(t, err)
		defer kv.RemoveWithPrefix("testMarshalLoadBalanceTask")
		value, err := kv.Load("testMarshalLoadBalanceTask")
		assert.Nil(t, err)

		task, err := taskScheduler.unmarshalTask(1008, value)
		assert.Nil(t, err)
		assert.Equal(t, task.msgType(), commonpb.MsgType_LoadBalanceSegments)
	})

	taskScheduler.Close()
}

func TestReloadTaskFromKV(t *testing.T) {
	refreshParams()
	kv, err := etcdkv.NewEtcdKV(Params.EtcdEndpoints, Params.MetaRootPath)
	assert.Nil(t, err)
	baseCtx, cancel := context.WithCancel(context.Background())
	taskScheduler := &TaskScheduler{
		ctx:              baseCtx,
		cancel:           cancel,
		client:           kv,
		triggerTaskQueue: NewTaskQueue(),
	}

	kvs := make(map[string]string)
	triggerTask := &loadCollectionTask{
		LoadCollectionRequest: &querypb.LoadCollectionRequest{
			Base: &commonpb.MsgBase{
				Timestamp: 1,
				MsgType:   commonpb.MsgType_LoadCollection,
			},
		},
	}
	triggerBlobs, err := triggerTask.marshal()
	assert.Nil(t, err)
	triggerTaskKey := fmt.Sprintf("%s/%d", triggerTaskPrefix, 100)
	kvs[triggerTaskKey] = string(triggerBlobs)

	activeTask := &loadSegmentTask{
		LoadSegmentsRequest: &querypb.LoadSegmentsRequest{
			Base: &commonpb.MsgBase{
				Timestamp: 2,
				MsgType:   commonpb.MsgType_LoadSegments,
			},
		},
	}
	activeBlobs, err := activeTask.marshal()
	assert.Nil(t, err)
	activeTaskKey := fmt.Sprintf("%s/%d", activeTaskPrefix, 101)
	kvs[activeTaskKey] = string(activeBlobs)

	stateKey := fmt.Sprintf("%s/%d", taskInfoPrefix, 100)
	kvs[stateKey] = strconv.Itoa(int(taskDone))
	err = kv.MultiSave(kvs)
	assert.Nil(t, err)

	taskScheduler.reloadFromKV()

	task := taskScheduler.triggerTaskQueue.PopTask()
	assert.Equal(t, taskDone, task.getState())
	assert.Equal(t, 1, len(task.getChildTask()))
}
