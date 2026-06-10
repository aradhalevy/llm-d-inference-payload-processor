# Requested Models Filter

Restricts the candidate models to those named in the request body.

It is registered as type `requested-models-filter` and runs as a modelselector filter.

## What it does

1. Reads the model field from the request body (`model` by default).
2. If the field is absent, an empty string, or an empty array, all candidates pass through unchanged.
3. If the field is a string or an array of strings, only candidates with a matching name are kept. An array means "choose among these": the scorers and picker select the best of the requested subset, and the model-selector plugin writes the winner back as a single string.
4. If no requested model is registered, or the field is malformed (not a string or an array of non-empty strings), all candidates are eliminated and the pipeline rejects the request with HTTP 400.

## Inputs consumed

- The configured model field of the request body.
- The candidate model list from the datalayer.

## Configuration

```json
{"modelField": "model"}
```

- `modelField` (optional): the request-body field holding the requested model name(s). Defaults to `model`.
