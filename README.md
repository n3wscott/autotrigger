# autotrigger

Automatically create triggers based on labels and annotations on Knative
Serving Services.

Looking for the `eventing.knative.dev/autotrigger` label:

```yaml
metadata:
  labels:
    eventing.knative.dev/autotrigger: "true"
```

And if this is found, the controller takes a look at the annotations:

```yaml
annotations:
  trigger.eventing.knative.dev/filter: |
    [{"type":"cloudevents.event.type"}]
```

Filter object is a json encoded string that turns into a list of objects:

```json
[
  {
    "broker": "knative-broker",
    "source": "source uri",
    "type": "cloudevents.event.type"
  }
]
```

`broker`, `source` and `type` are optional. `broker` defaults to "default".
`source` defaults to "Any". `type` defaults to "Any".

If you want to select on all events passing through the default broker:

```yaml
annotations:
  trigger.eventing.knative.dev/filter: "[{}]"
```

You can add more than one filter:

```yaml
annotations:
  trigger.eventing.knative.dev/filter: |
    [{"type":"cloudevents.event.foo"},{"type":"cloudevents.event.bar"}]
```

Full example:

```yaml
apiVersion: serving.knative.dev/v1alpha1
kind: Service
metadata:
  name: auto-event-display
  labels:
    eventing.knative.dev/autotrigger: "true"
  annotations:
    trigger.eventing.knative.dev/filter: |
      [{"type":"botless.slack.message"},{"type":"botless.bot.command"}]
spec:
  runLatest:
    configuration:
      revisionTemplate:
        spec:
          container:
            image: github.com/knative/eventing-sources/cmd/event_display
```

### Known Issues:

- Autotrigger controller does not clean up triggers if the
  `eventing.knative.dev/autotrigger` label is removed.
  - you could remove the filters, then the annotation and that will clean them
    up.
