# AutoTrigger

Magically create Triggers based on labels/annotations on any [Knative](https://knative.dev)
[Addressable](https://godoc.org/github.com/knative/pkg/apis/duck/v1#Addressable) resource!

## Install

```shell
kubectl apply -f https://github.com/n3wscott/autotrigger/releases/download/v0.1.0/release.yaml
```

## Usage

AutoTrigger automatically create triggers based on labels and annotations on Knative
[Addressable](https://godoc.org/github.com/knative/pkg/apis/duck/v1#Addressable) resources.

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

### Full Example

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: auto-event-display
  labels:
    eventing.knative.dev/autotrigger: "true"
  annotations:
    trigger.eventing.knative.dev/filter: |
      [{"type":"botless.slack.message"},{"type":"botless.bot.command"}]
spec:
  template:
    spec:
      containers:
        - image: github.com/knative/eventing-sources/cmd/event_display
```

### Known Issues:

- Autotrigger controller does not clean up triggers if the
  `eventing.knative.dev/autotrigger` label is removed.
  - you could remove the filters, then the annotation and that will clean them
    up.
