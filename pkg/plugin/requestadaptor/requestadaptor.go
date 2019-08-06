package requestadaptor

import (
	"fmt"

	"github.com/megaease/easegateway/pkg/context"
	"github.com/megaease/easegateway/pkg/object/httppipeline"
	"github.com/megaease/easegateway/pkg/util/httpheader"
)

const (
	// Kind is the kind of RequestAdaptor.
	Kind = "RequestAdaptor"
)

func init() {
	httppipeline.Register(&httppipeline.PluginRecord{
		Kind:            Kind,
		DefaultSpecFunc: DefaultSpec,
		NewFunc:         New,
		Results:         nil,
	})
}

// DefaultSpec returns default spec.
func DefaultSpec() *Spec {
	return &Spec{}
}

type (
	// RequestAdaptor is plugin RequestAdaptor.
	RequestAdaptor struct {
		spec *Spec

		pathAdaptor *pathAdaptor
	}

	// Spec is HTTPAdaptor Spec.
	Spec struct {
		httppipeline.PluginMeta `yaml:",inline"`

		Method string                `yaml:"method" v:"omitempty,httpmethod"`
		Path   *pathAdaptorSpec      `yaml:"path"`
		Header *httpheader.AdaptSpec `yaml:"header"`
	}
)

// New creates an HTTPAdaptor.
func New(spec *Spec) *RequestAdaptor {
	var pathAdaptor *pathAdaptor
	if spec.Path != nil {
		pathAdaptor = newPathAdaptor(spec.Path)
	}

	return &RequestAdaptor{
		spec:        spec,
		pathAdaptor: pathAdaptor,
	}
}

// Handle adapts request.
func (ra *RequestAdaptor) Handle(ctx context.HTTPContext) string {
	r := ctx.Request()
	method, path, header := r.Method(), r.Path(), r.Header()

	if ra.spec.Method != "" && ra.spec.Method != method {
		ctx.AddTag(fmt.Sprintf("requestAdaptor: method %s adapted to %s",
			method, ra.spec.Method))
		r.SetMethod(ra.spec.Method)
	}
	if ra.pathAdaptor != nil {
		adaptedPath := ra.pathAdaptor.Adapt(path)
		if adaptedPath != path {
			ctx.AddTag(fmt.Sprintf("requestAdaptor: path %s adapted to %s",
				path, adaptedPath))
		}
		r.SetPath(adaptedPath)
	}
	if ra.spec.Header != nil {
		header.Adapt(ra.spec.Header)
	}

	return ""
}
