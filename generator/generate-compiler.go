// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"strings"
)

func (classes *ClassCollection) generateCompiler(packageName string, license string) string {
	code := CodeBuilder{}
	code.AddLine(license)
	code.AddLine("// THIS FILE IS AUTOMATICALLY GENERATED.")
	code.AddLine()
	code.AddLine("package main")
	code.AddLine()
	code.AddLine("import (")
	code.AddLine("\"fmt\"")
	code.AddLine("\"log\"")
	code.AddLine("pb \"openapi\"")
	code.AddLine(")")
	code.AddLine()
	code.AddLine("func version() string {")
	code.AddLine("  return \"OpenAPIv2\"")
	code.AddLine("}")
	code.AddLine()

	classNames := classes.sortedClassNames()
	for _, className := range classNames {
		code.AddLine("func build%s(in interface{}) *pb.%s {", className, className)

		classModel := classes.ClassModels[className]
		parentClassName := className

		if classModel.IsStringArray {
			code.AddLine("value, ok := in.(string)")
			code.AddLine("x := &pb.TypeItem{}")
			code.AddLine("if ok {")
			code.AddLine("x.Value = make([]string, 0)")
			code.AddLine("x.Value = append(x.Value, value)")
			code.AddLine("} else {")
			code.AddLine("log.Printf(\"unexpected: %+v\", in)")
			code.AddLine("}")
			code.AddLine("return x")
			code.AddLine("}")
			continue
		}

		if classModel.IsBlob {
			code.AddLine("x := &pb.Any{}")
			code.AddLine("x.Value = fmt.Sprintf(\"%%+v\", in)")
			code.AddLine("return x")
			code.AddLine("}")
			continue
		}

		code.AddLine("m, keys, ok := unpackMap(in)")
		code.AddLine("if (!ok) {")
		code.AddLine("log.Printf(\"unexpected argument to build%s: %%+v\", in)", className)
		code.AddLine("log.Printf(\"%%d\\n\", len(m))")
		code.AddLine("log.Printf(\"%%+v\\n\", keys)")
		code.AddLine("return nil")
		code.AddLine("}")
		oneOfWrapper := classModel.OneOfWrapper

		propertyNames := classModel.sortedPropertyNames()
		if len(classModel.Required) > 0 {
			// verify that map includes all required keys
			keyString := ""
			for _, k := range classModel.Required {
				if keyString != "" {
					keyString += ","
				}
				keyString += "\""
				keyString += k
				keyString += "\""
			}
			code.AddLine("requiredKeys := []string{%s}", keyString)
			code.AddLine("if !mapContainsAllKeys(m, requiredKeys) {")
			code.AddLine("return nil")
			code.AddLine("}")
		}

		if !classModel.Open {
			// verify that map has no unspecified keys
			keyString := ""
			for _, property := range classModel.Properties {
				if keyString != "" {
					keyString += ","
				}
				keyString += "\""
				keyString += property.Name
				keyString += "\""
			}
			// verify that map includes all required keys
			code.AddLine("allowedKeys := []string{%s}", keyString)
			code.AddLine("if !mapContainsOnlyKeys(m, allowedKeys) {")
			code.AddLine("return nil")
			code.AddLine("}")
		}

		code.AddLine("  x := &pb.%s{}", className)

		var fieldNumber = 0
		for _, propertyName := range propertyNames {
			propertyModel := classModel.Properties[propertyName]
			fieldNumber += 1
			propertyType := propertyModel.Type
			if propertyType == "int" {
				propertyType = "int64"
			}
			var displayName = propertyName
			if displayName == "$ref" {
				displayName = "_ref"
			}
			if displayName == "$schema" {
				displayName = "_schema"
			}
			displayName = camelCaseToSnakeCase(displayName)

			var line = fmt.Sprintf("%s %s = %d;", propertyType, displayName, fieldNumber)
			if propertyModel.Repeated {
				line = "repeated " + line
			}
			code.AddLine("// " + line)

			fieldName := strings.Title(propertyName)
			if propertyName == "$ref" {
				fieldName = "XRef"
			}

			classModel, classFound := classes.ClassModels[propertyType]
			if classFound {
				if propertyModel.Repeated {
					code.AddLine("if mapHasKey(m, \"%s\") {", propertyName)
					code.AddLine("// repeated class %s", classModel.Name)
					code.AddLine("x.%s = make([]*pb.%s, 0)", fieldName, classModel.Name)
					code.AddLine("a, ok := m[\"%s\"].([]interface{})", propertyName)
					code.AddLine("if ok {")
					code.AddLine("for _, item := range a {")
					code.AddLine("x.%s = append(x.%s, build%s(item))", fieldName, fieldName, classModel.Name)
					code.AddLine("}")
					code.AddLine("}")
					code.AddLine("}")
				} else {
					if oneOfWrapper {
						code.AddLine("{")
						code.AddLine("t := build%s(m)", classModel.Name)
						code.AddLine("if t != nil {")
						code.AddLine("x.Oneof = &pb.%s_%s{%s: t}", parentClassName, classModel.Name, classModel.Name)
						code.AddLine("}")
						code.AddLine("}")
					} else {
						code.AddLine("if mapHasKey(m, \"%s\") {", propertyName)
						code.AddLine("x.%s = build%s(m[\"%v\"])", fieldName, classModel.Name, propertyName)
						code.AddLine("}")
					}
				}
			} else if propertyType == "string" {
				if propertyModel.Repeated {
					code.AddLine("if mapHasKey(m, \"%s\") {", propertyName)
					code.AddLine("v, ok := m[\"%v\"].([]interface{})", propertyName)
					code.AddLine("if ok {")
					code.AddLine("x.%s = convertInterfaceArrayToStringArray(v)", fieldName)
					code.AddLine("} else {")
					code.AddLine(" log.Printf(\"unexpected: %%+v\", m[\"%v\"])", propertyName)
					code.AddLine("}")
					code.AddLine("}")
				} else {
					code.AddLine("if mapHasKey(m, \"%s\") {", propertyName)
					code.AddLine("x.%s = m[\"%v\"].(string)", fieldName, propertyName)
					code.AddLine("}")
				}
			} else if propertyType == "float" {
				code.AddLine("if mapHasKey(m, \"%s\") {", propertyName)
				code.AddLine("x.%s = m[\"%v\"].(float64)", fieldName, propertyName)
				code.AddLine("}")
			} else if propertyType == "int64" {
				code.AddLine("if mapHasKey(m, \"%s\") {", propertyName)
				code.AddLine("x.%s = m[\"%v\"].(int64)", fieldName, propertyName)
				code.AddLine("}")
			} else if propertyType == "bool" {
				code.AddLine("if mapHasKey(m, \"%s\") {", propertyName)
				code.AddLine("x.%s = m[\"%v\"].(bool)", fieldName, propertyName)
				code.AddLine("}")
			} else {
				isMap, mapTypeName := mapTypeInfo(propertyType)
				if isMap {
					code.AddLine("// MAP: %s %s", mapTypeName, propertyModel.Pattern)
					if mapTypeName == "string" {
						code.AddLine("x.%s = make(map[string]string, 0)", fieldName)
					} else {
						code.AddLine("x.%s = make(map[string]*pb.%s, 0)", fieldName, mapTypeName)
					}
					code.AddLine("for k, v := range m {")
					if propertyModel.Pattern != "" {
						code.AddLine("if patternMatches(\"%s\", k) {", propertyModel.Pattern)
					}
					if mapTypeName == "string" {
						code.AddLine("x.%s[k] = v.(string)", fieldName)
					} else {
						code.AddLine("x.%s[k] = build%v(v)", fieldName, mapTypeName)
					}
					if propertyModel.Pattern != "" {
						code.AddLine("}")
					}
					code.AddLine("}")
				} else {
					code.AddLine("// TODO: %s", propertyType)
				}
			}
		}
		code.AddLine("  return x")
		code.AddLine("}\n")
	}
	return code.Text()
}
