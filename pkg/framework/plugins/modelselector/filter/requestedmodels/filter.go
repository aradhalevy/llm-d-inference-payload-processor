/*
Copyright 2026 The llm-d Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package requestedmodels implements a modelselector filter that restricts the
// candidate models to those named in the request body.
//
// For detailed behavioral intent and configuration, see the package README.
package requestedmodels

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/log"

	logutil "github.com/llm-d/llm-d-inference-payload-processor/pkg/common/observability/logging"
	"github.com/llm-d/llm-d-inference-payload-processor/pkg/framework/interface/datalayer"
	"github.com/llm-d/llm-d-inference-payload-processor/pkg/framework/interface/modelselector"
	"github.com/llm-d/llm-d-inference-payload-processor/pkg/framework/interface/plugin"
	"github.com/llm-d/llm-d-inference-payload-processor/pkg/framework/interface/requesthandling"
)

const (
	// RequestedModelsFilterType is the registered name of the requested-models filter plugin.
	RequestedModelsFilterType = "requested-models-filter"

	// defaultModelField is the request-body field inspected when none is configured.
	defaultModelField = "model"
)

// compile-time type validation
var _ modelselector.Filter = &RequestedModelsFilter{}

// RequestedModelsFilterConfig defines the JSON configuration structure for the plugin.
type RequestedModelsFilterConfig struct {
	// ModelField is the request-body field that holds the requested model name(s).
	// Defaults to "model" when empty.
	ModelField string `json:"modelField,omitempty"`
}

// RequestedModelsFilterFactory defines the factory function for RequestedModelsFilter.
func RequestedModelsFilterFactory(name string, rawParameters json.RawMessage, _ plugin.Handle) (plugin.Plugin, error) {
	var config RequestedModelsFilterConfig

	if len(rawParameters) > 0 {
		if err := json.Unmarshal(rawParameters, &config); err != nil {
			return nil, fmt.Errorf("failed to parse the parameters of the '%s' filter - %w", RequestedModelsFilterType, err)
		}
	}

	return NewRequestedModelsFilter(config.ModelField).WithName(name), nil
}

// NewRequestedModelsFilter initializes a new RequestedModelsFilter and returns its pointer.
// An empty modelField defaults to "model".
func NewRequestedModelsFilter(modelField string) *RequestedModelsFilter {
	if modelField == "" {
		modelField = defaultModelField
	}

	return &RequestedModelsFilter{
		typedName:  plugin.TypedName{Type: RequestedModelsFilterType, Name: RequestedModelsFilterType},
		modelField: modelField,
	}
}

// RequestedModelsFilter restricts the candidate models to those named in the request body.
type RequestedModelsFilter struct {
	typedName  plugin.TypedName
	modelField string
}

// TypedName returns the type and name tuple of this plugin instance.
func (f *RequestedModelsFilter) TypedName() plugin.TypedName {
	return f.typedName
}

// WithName sets the name of the plugin instance.
func (f *RequestedModelsFilter) WithName(name string) *RequestedModelsFilter {
	f.typedName.Name = name
	return f
}

// Filter returns the candidate models whose name was requested by the request body.
func (f *RequestedModelsFilter) Filter(ctx context.Context, _ *plugin.CycleState, request *requesthandling.InferenceRequest, models []datalayer.Model) []datalayer.Model {
	logger := log.FromContext(ctx)

	requested, ok := requestedModelNames(request.Body[f.modelField])
	if !ok {
		logger.V(logutil.VERBOSE).Info("malformed model field in request body, eliminating all candidates", "field", f.modelField)
		return nil
	}
	if requested.Len() == 0 {
		// No model named in the request: nothing to constrain, let all candidates pass.
		logger.V(logutil.VERBOSE).Info("no model requested, passing all candidates through", "field", f.modelField)
		return models
	}

	filtered := make([]datalayer.Model, 0, min(len(models), requested.Len()))
	for _, model := range models {
		if requested.Has(model.GetName()) {
			filtered = append(filtered, model)
		}
	}

	if len(filtered) == 0 {
		logger.V(logutil.VERBOSE).Info("requested model(s) not registered in datalayer", "requested", requested.UnsortedList())
	} else {
		logger.V(logutil.DEBUG).Info("requested-models filter applied", "requested", requested.UnsortedList())
	}

	return filtered
}

// requestedModelNames extracts the set of requested model names from a request-body
// model field, which may be a single string or an array of non-empty strings.
// An absent field (nil), an empty string, or an empty array yield an empty set,
// meaning the request does not constrain the candidates. Any other shape —
// including non-string or empty-string array elements — is malformed and
// reported by the second return value being false.
func requestedModelNames(raw any) (sets.Set[string], bool) {
	names := sets.New[string]()

	switch value := raw.(type) {
	case nil:
	case string:
		if value != "" {
			names.Insert(value)
		}
	case []any:
		for _, elem := range value {
			name, isString := elem.(string)
			if !isString || name == "" {
				return nil, false
			}
			names.Insert(name)
		}
	default:
		return nil, false
	}

	return names, true
}
