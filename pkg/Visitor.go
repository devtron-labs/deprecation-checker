/*
 * Copyright (c) 2021 Devtron Labs
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package pkg

import (
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"strings"
)

type SchemaSettings struct {
	MultiError bool
}

func VisitJSON(key string, schema *openapi3.Schema, value interface{}, settings SchemaSettings) openapi3.MultiError {
	return visitJSON(key, schema, value, settings)
}

func visitJSON(key string, schema *openapi3.Schema, value interface{}, settings SchemaSettings) openapi3.MultiError {
	var me openapi3.MultiError
	switch value := value.(type) {
	case nil, bool, float64, string, int64:
		if strings.Index(strings.ToLower(schema.Description), "deprecated") >= 0 {
			schemaError := &SchemaError{
				Value:       "",
				Schema:      schema,
				SchemaField: key,
				Reason:      schema.Description,
			}
			me = append(me, schemaError)
		}
		return me
	case []interface{}:
		return visitJSONArray(key, schema, value, settings)
	case map[string]interface{}:
		return visitJSONObject(key, schema, value, settings)
	default:
		schemaError := &SchemaError{
			Value:       value,
			Schema:      schema,
			SchemaField: key,
			Reason:      fmt.Sprintf("unhandled key %s", key),
		}
		me = append(me, schemaError)
		return me
	}
}

func visitJSONArray(key string, schema *openapi3.Schema, object []interface{}, settings SchemaSettings) openapi3.MultiError {
	var me openapi3.MultiError
	for _, obj := range object {
		schemaError := visitJSON(key, schema.Items.Value, obj, settings)
		if len(schemaError) != 0 {
			me = append(me, schemaError...)
		}
	}
	return me
}

func visitJSONObject(key string, schema *openapi3.Schema, object map[string]interface{}, settings SchemaSettings) openapi3.MultiError {
	var me openapi3.MultiError
	if strings.Index(strings.ToLower(schema.Description), "deprecated") >= 0 {
		schemaError := &SchemaError{
			Value:       "",
			Schema:      schema,
			SchemaField: key,
			Reason:      schema.Description,
		}
		me = append(me, schemaError)
		if !settings.MultiError {
			return me
		}
	}
	for k, v := range object {
		if s, ok := schema.Properties[k]; ok {
			//fmt.Printf("found key %s\n", k)
			schemaError := visitJSON(k, s.Value, v, settings)
			if len(schemaError) != 0 {
				me = append(me, schemaError...)
				if !settings.MultiError {
					return me
				}
			}
		} else {
			//fmt.Printf("not found key %s\n", k)
		}
	}
	return me
}
