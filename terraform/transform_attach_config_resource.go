package terraform

import (
	"log"

	"github.com/hashicorp/terraform/configs"
)

// GraphNodeAttachResourceConfig is an interface that must be implemented by nodes
// that want resource configurations attached.
type GraphNodeAttachResourceConfig interface {
	GraphNodeResource

	// Sets the configuration
	AttachResourceConfig(*configs.Resource)
}

// AttachResourceConfigTransformer goes through the graph and attaches
// resource configuration structures to nodes that implement
// GraphNodeAttachManagedResourceConfig or GraphNodeAttachDataResourceConfig.
//
// The attached configuration structures are directly from the configuration.
// If they're going to be modified, a copy should be made.
type AttachResourceConfigTransformer struct {
	Config *configs.Config // Config is the root node in the config tree
}

func (t *AttachResourceConfigTransformer) Transform(g *Graph) error {
	log.Printf("[TRACE] AttachResourceConfigTransformer: Beginning...")

	// Go through and find GraphNodeAttachResource
	for _, v := range g.Vertices() {
		// Only care about GraphNodeAttachResource implementations
		arn, ok := v.(GraphNodeAttachResourceConfig)
		if !ok {
			continue
		}

		// Determine what we're looking for
		addr := arn.ResourceAddr()
		log.Printf("[TRACE] AttachResourceConfigTransformer: Request for configuration on %s", addr.String())

		// Get the configuration.
		config := t.Config.DescendentForInstance(addr.Module)
		if config == nil {
			continue
		}

		for _, r := range config.Module.ManagedResources {
			rAddr := r.Addr()

			if rAddr != addr.Resource {
				// Not the same resource
				continue
			}

			log.Printf("[TRACE] AttachResourceConfigTransformer: Attaching to %s: %#v", addr.String(), r)
			arn.AttachResourceConfig(r)
		}
		for _, r := range config.Module.DataResources {
			rAddr := r.Addr()

			if rAddr != addr.Resource {
				// Not the same resource
				continue
			}

			log.Printf("[TRACE] AttachResourceConfigTransformer: Attaching to %s: %#v", addr.String(), r)
			arn.AttachResourceConfig(r)
		}
	}

	return nil
}
