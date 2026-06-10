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

package requestedmodels

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/llm-d/llm-d-inference-payload-processor/pkg/framework/interface/datalayer"
	"github.com/llm-d/llm-d-inference-payload-processor/pkg/framework/interface/requesthandling"
)

func candidateModels(names ...string) []datalayer.Model {
	models := make([]datalayer.Model, 0, len(names))
	for _, n := range names {
		models = append(models, datalayer.NewModel(n))
	}
	return models
}

func names(models []datalayer.Model) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		out = append(out, m.GetName())
	}
	sort.Strings(out)
	return out
}

func requestWithModel(field string, value any) *requesthandling.InferenceRequest {
	r := requesthandling.NewInferenceRequest()
	if value != nil {
		r.Body[field] = value
	}
	return r
}

func TestRequestedModelsFilterFactory(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
		rawParams  json.RawMessage
		wantErr    bool
		wantField  string
	}{
		{
			name:       "empty params defaults to model field",
			pluginName: "my-filter",
			rawParams:  json.RawMessage(``),
			wantField:  defaultModelField,
		},
		{
			name:       "custom model field",
			pluginName: "my-filter",
			rawParams:  json.RawMessage(`{"modelField":"requestedModel"}`),
			wantField:  "requestedModel",
		},
		{
			name:       "invalid JSON",
			pluginName: "my-filter",
			rawParams:  json.RawMessage(`{invalid`),
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := RequestedModelsFilterFactory(tt.pluginName, tt.rawParams, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			f := p.(*RequestedModelsFilter)
			if f.modelField != tt.wantField {
				t.Errorf("modelField = %s, want %s", f.modelField, tt.wantField)
			}
			if got := f.TypedName().Name; got != tt.pluginName {
				t.Errorf("Name = %s, want %s", got, tt.pluginName)
			}
			if got := f.TypedName().Type; got != RequestedModelsFilterType {
				t.Errorf("Type = %s, want %s", got, RequestedModelsFilterType)
			}
		})
	}
}

func TestRequestedModelsFilter_Filter(t *testing.T) {
	registered := []string{"qwen3", "llama3", "mistral"}

	tests := []struct {
		name      string
		modelBody any // value stored at request.Body["model"]
		want      []string
	}{
		{
			name:      "single registered model keeps only it",
			modelBody: "qwen3",
			want:      []string{"qwen3"},
		},
		{
			name:      "single unregistered model yields empty (pipeline error)",
			modelBody: "gpt-4",
			want:      []string{},
		},
		{
			name:      "array keeps only the registered ones",
			modelBody: []any{"qwen3", "mistral", "gpt-4"},
			want:      []string{"mistral", "qwen3"},
		},
		{
			name:      "missing model field passes all through",
			modelBody: nil,
			want:      registered,
		},
		{
			name:      "empty string model passes all through",
			modelBody: "",
			want:      registered,
		},
		{
			name:      "non-string model field yields empty (malformed)",
			modelBody: 42,
			want:      []string{},
		},
		{
			name:      "array with non-string element yields empty (malformed)",
			modelBody: []any{"qwen3", 42},
			want:      []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewRequestedModelsFilter("")
			req := requestWithModel(defaultModelField, tt.modelBody)

			got := names(f.Filter(context.Background(), nil, req, candidateModels(registered...)))

			want := append([]string{}, tt.want...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Fatalf("Filter() = %v, want %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("Filter() = %v, want %v", got, want)
					break
				}
			}
		})
	}
}

func TestRequestedModelsFilter_CustomModelField(t *testing.T) {
	f := NewRequestedModelsFilter("requestedModel")
	req := requestWithModel("requestedModel", "llama3")

	got := names(f.Filter(context.Background(), nil, req, candidateModels("qwen3", "llama3")))
	if len(got) != 1 || got[0] != "llama3" {
		t.Errorf("Filter() = %v, want [llama3]", got)
	}
}
