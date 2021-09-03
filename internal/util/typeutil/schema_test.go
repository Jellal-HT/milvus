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

package typeutil

import (
	"testing"

	"github.com/milvus-io/milvus/internal/proto/commonpb"
	"github.com/milvus-io/milvus/internal/proto/schemapb"
	"github.com/stretchr/testify/assert"
)

func TestSchema(t *testing.T) {
	schema := &schemapb.CollectionSchema{
		Name:        "testColl",
		Description: "",
		AutoID:      false,
		Fields: []*schemapb.FieldSchema{
			{
				FieldID:      100,
				Name:         "field_int8",
				IsPrimaryKey: false,
				Description:  "",
				DataType:     2,
			},
			{
				FieldID:      101,
				Name:         "field_int16",
				IsPrimaryKey: false,
				Description:  "",
				DataType:     3,
			},
			{
				FieldID:      102,
				Name:         "field_int32",
				IsPrimaryKey: false,
				Description:  "",
				DataType:     4,
			},
			{
				FieldID:      103,
				Name:         "field_int64",
				IsPrimaryKey: true,
				Description:  "",
				DataType:     5,
			},
			{
				FieldID:      104,
				Name:         "field_float_vector",
				IsPrimaryKey: false,
				Description:  "",
				DataType:     101,
				TypeParams: []*commonpb.KeyValuePair{
					{
						Key:   "dim",
						Value: "128",
					},
				},
				IndexParams: []*commonpb.KeyValuePair{
					{
						Key:   "nlist",
						Value: "128",
					},
				},
			},
			{
				FieldID:      105,
				Name:         "field_binary_vector",
				IsPrimaryKey: false,
				Description:  "",
				DataType:     100,
				TypeParams: []*commonpb.KeyValuePair{
					{
						Key:   "dim",
						Value: "128",
					},
				},
			},
		},
	}

	t.Run("EstimateSizePerRecord", func(t *testing.T) {
		size, err := EstimateSizePerRecord(schema)
		assert.Equal(t, 543, size)
		assert.Nil(t, err)
	})

	t.Run("SchemaHelper", func(t *testing.T) {
		helper, err := CreateSchemaHelper(schema)
		assert.Nil(t, err)

		field, err := helper.GetPrimaryKeyField()
		assert.Nil(t, err)
		assert.Equal(t, "field_int64", field.Name)

		field1, err := helper.GetFieldFromName("field_int8")
		assert.Nil(t, err)
		assert.Equal(t, "field_int8", field1.Name)

		field2, err := helper.GetFieldFromID(102)
		assert.Nil(t, err)
		assert.Equal(t, "field_int32", field2.Name)

		dim, err := helper.GetVectorDimFromID(104)
		assert.Nil(t, err)
		assert.Equal(t, 128, dim)
		dim1, err := helper.GetVectorDimFromID(105)
		assert.Nil(t, err)
		assert.Equal(t, 128, dim1)
		_, err = helper.GetVectorDimFromID(103)
		assert.NotNil(t, err)
	})

	t.Run("Type", func(t *testing.T) {
		assert.False(t, IsVectorType(schemapb.DataType_Bool))
		assert.False(t, IsVectorType(schemapb.DataType_Int8))
		assert.False(t, IsVectorType(schemapb.DataType_Int16))
		assert.False(t, IsVectorType(schemapb.DataType_Int32))
		assert.False(t, IsVectorType(schemapb.DataType_Int64))
		assert.False(t, IsVectorType(schemapb.DataType_Float))
		assert.False(t, IsVectorType(schemapb.DataType_Double))
		assert.False(t, IsVectorType(schemapb.DataType_String))
		assert.True(t, IsVectorType(schemapb.DataType_BinaryVector))
		assert.True(t, IsVectorType(schemapb.DataType_FloatVector))

		assert.False(t, IsIntegerType(schemapb.DataType_Bool))
		assert.True(t, IsIntegerType(schemapb.DataType_Int8))
		assert.True(t, IsIntegerType(schemapb.DataType_Int16))
		assert.True(t, IsIntegerType(schemapb.DataType_Int32))
		assert.True(t, IsIntegerType(schemapb.DataType_Int64))
		assert.False(t, IsIntegerType(schemapb.DataType_Float))
		assert.False(t, IsIntegerType(schemapb.DataType_Double))
		assert.False(t, IsIntegerType(schemapb.DataType_String))
		assert.False(t, IsIntegerType(schemapb.DataType_BinaryVector))
		assert.False(t, IsIntegerType(schemapb.DataType_FloatVector))

		assert.False(t, IsFloatingType(schemapb.DataType_Bool))
		assert.False(t, IsFloatingType(schemapb.DataType_Int8))
		assert.False(t, IsFloatingType(schemapb.DataType_Int16))
		assert.False(t, IsFloatingType(schemapb.DataType_Int32))
		assert.False(t, IsFloatingType(schemapb.DataType_Int64))
		assert.True(t, IsFloatingType(schemapb.DataType_Float))
		assert.True(t, IsFloatingType(schemapb.DataType_Double))
		assert.False(t, IsFloatingType(schemapb.DataType_String))
		assert.False(t, IsFloatingType(schemapb.DataType_BinaryVector))
		assert.False(t, IsFloatingType(schemapb.DataType_FloatVector))
	})
}