// The MIT License
//
// Copyright (c) 2020 Temporal Technologies Inc.  All rights reserved.
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"

	annpb "go.temporal.io/api/annotations/v1"
	"go.temporal.io/api/operatorservice/v1"
	"go.temporal.io/api/workflowservice/v1"
)

func checkAnnotations(protoFile, serviceName string, serverType reflect.Type) error {
	r, err := gzip.NewReader(bytes.NewReader(proto.FileDescriptor(protoFile)))
	if err != nil {
		return err
	}
	buf, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	var d descriptor.FileDescriptorProto
	if err := proto.Unmarshal(buf, &d); err != nil {
		return err
	}

	for _, service := range d.Service {
		if *service.Name != serviceName {
			continue
		}
		for _, method := range service.Method {
			name := *method.Name
			v, _ := proto.GetExtension(method.Options, annpb.E_Method)
			ann, _ := v.(*annpb.Method)
			if err := validateAnnotation(serverType, name, ann); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateAnnotation(serverType reflect.Type, name string, ann *annpb.Method) error {
	if ann == nil {
		return fmt.Errorf("annotation for %s missing", name)
	}
	if ann.Scope == annpb.SCOPE_UNSPECIFIED {
		return fmt.Errorf("scope annotation for %s missing", name)
	}
	if ann.Access == annpb.ACCESS_UNSPECIFIED {
		return fmt.Errorf("access annotation for %s missing", name)
	}

	// check namespace/scope consistency
	refMethod, ok := serverType.MethodByName(name)
	if !ok {
		return fmt.Errorf("can't find method %s in server type", name)
	}

	if refMethod.Type.NumIn() >= 2 {
		// regular method (not streaming)
		reqType := refMethod.Type.In(1).Elem()
		_, hasNamespace := reqType.FieldByName("Namespace")
		if ann.Scope == annpb.SCOPE_CLUSTER && hasNamespace {
			return fmt.Errorf("found Namespace field in request for SCOPE_CLUSTER method %s", name)
		} else if ann.Scope == annpb.SCOPE_NAMESPACE && !hasNamespace {
			return fmt.Errorf("didn't find Namespace field in request for SCOPE_NAMESPACE method %s", name)
		}
	}

	return nil
}

func main() {
	err := checkAnnotations(
		"temporal/api/workflowservice/v1/service.proto",
		"WorkflowService",
		reflect.TypeOf((*workflowservice.WorkflowServiceServer)(nil)).Elem(),
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = checkAnnotations(
		"temporal/api/operatorservice/v1/service.proto",
		"OperatorService",
		reflect.TypeOf((*operatorservice.OperatorServiceServer)(nil)).Elem(),
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
