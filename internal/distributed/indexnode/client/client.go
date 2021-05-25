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

package grpcindexnodeclient

import (
	"context"
	"fmt"
	"time"

	"github.com/milvus-io/milvus/internal/log"
	"github.com/milvus-io/milvus/internal/proto/milvuspb"
	"github.com/milvus-io/milvus/internal/util/retry"
	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/milvus-io/milvus/internal/proto/commonpb"
	"github.com/milvus-io/milvus/internal/proto/indexpb"
	"github.com/milvus-io/milvus/internal/proto/internalpb"
)

type Client struct {
	grpcClient indexpb.IndexNodeClient
	conn       *grpc.ClientConn
	ctx        context.Context

	address string

	timeout   time.Duration
	reconnTry int
	recallTry int
}

func NewClient(address string, timeout time.Duration) (*Client, error) {
	if address == "" {
		return nil, fmt.Errorf("address is empty")
	}
	return &Client{
		address:   address,
		ctx:       context.Background(),
		timeout:   timeout,
		recallTry: 3,
		reconnTry: 10,
	}, nil
}

func (c *Client) Init() error {
	tracer := opentracing.GlobalTracer()
	connectGrpcFunc := func() error {
		ctx, cancelFunc := context.WithTimeout(c.ctx, c.timeout)
		defer cancelFunc()
		log.Debug("IndexNodeClient try connect ", zap.String("address", c.address))
		conn, err := grpc.DialContext(ctx, c.address, grpc.WithInsecure(), grpc.WithBlock(),
			grpc.WithUnaryInterceptor(
				otgrpc.OpenTracingClientInterceptor(tracer)),
			grpc.WithStreamInterceptor(
				otgrpc.OpenTracingStreamClientInterceptor(tracer)))
		if err != nil {
			return err
		}
		c.conn = conn
		return nil
	}
	err := retry.Retry(100000, time.Millisecond*200, connectGrpcFunc)
	if err != nil {
		log.Debug("IndexNodeClient try connect failed", zap.Error(err))
		return err
	}
	log.Debug("IndexNodeClient try connect success", zap.String("address", c.address))
	c.grpcClient = indexpb.NewIndexNodeClient(c.conn)
	return nil
}

func (c *Client) reconnect() error {
	tracer := opentracing.GlobalTracer()
	var err error
	connectGrpcFunc := func() error {
		ctx, cancelFunc := context.WithTimeout(c.ctx, c.timeout)
		defer cancelFunc()
		log.Debug("IndexNodeClient try reconnect ", zap.String("address", c.address))
		conn, err := grpc.DialContext(ctx, c.address, grpc.WithInsecure(), grpc.WithBlock(),
			grpc.WithUnaryInterceptor(
				otgrpc.OpenTracingClientInterceptor(tracer)),
			grpc.WithStreamInterceptor(
				otgrpc.OpenTracingStreamClientInterceptor(tracer)))
		if err != nil {
			return err
		}
		c.conn = conn
		return nil
	}

	err = retry.Retry(c.reconnTry, 500*time.Millisecond, connectGrpcFunc)
	if err != nil {
		log.Debug("IndexNodeClient try reconnect failed", zap.Error(err))
		return err
	}
	log.Debug("IndexNodeClient try reconnect success", zap.String("address", c.address))
	c.grpcClient = indexpb.NewIndexNodeClient(c.conn)
	return nil
}

func (c *Client) recall(caller func() (interface{}, error)) (interface{}, error) {
	ret, err := caller()
	if err == nil {
		return ret, nil
	}
	for i := 0; i < c.recallTry; i++ {
		err = c.reconnect()
		if err == nil {
			ret, err = caller()
			if err == nil {
				return ret, nil
			}
		}
	}
	return ret, err
}

func (c *Client) Start() error {
	return nil
}

func (c *Client) Stop() error {
	return nil
}

// Register dummy
func (c *Client) Register() error {
	return nil
}

func (c *Client) GetComponentStates(ctx context.Context) (*internalpb.ComponentStates, error) {
	ret, err := c.recall(func() (interface{}, error) {
		return c.grpcClient.GetComponentStates(ctx, &internalpb.GetComponentStatesRequest{})
	})
	return ret.(*internalpb.ComponentStates), err
}

func (c *Client) GetTimeTickChannel(ctx context.Context) (*milvuspb.StringResponse, error) {
	ret, err := c.recall(func() (interface{}, error) {
		return c.grpcClient.GetTimeTickChannel(ctx, &internalpb.GetTimeTickChannelRequest{})
	})
	return ret.(*milvuspb.StringResponse), err
}

func (c *Client) GetStatisticsChannel(ctx context.Context) (*milvuspb.StringResponse, error) {
	ret, err := c.recall(func() (interface{}, error) {
		return c.grpcClient.GetStatisticsChannel(ctx, &internalpb.GetStatisticsChannelRequest{})
	})
	return ret.(*milvuspb.StringResponse), err
}

func (c *Client) CreateIndex(ctx context.Context, req *indexpb.CreateIndexRequest) (*commonpb.Status, error) {
	ret, err := c.recall(func() (interface{}, error) {
		return c.grpcClient.CreateIndex(ctx, req)
	})
	return ret.(*commonpb.Status), err
}