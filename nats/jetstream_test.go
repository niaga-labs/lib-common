package nats

import (
	"strings"
	"testing"
)

func TestDefaultStreamsDoNotIncludeCatchAllEventsStream(t *testing.T) {
	for _, stream := range DefaultStreams {
		for _, subject := range stream.Subjects {
			if subject == "events.>" {
				t.Fatalf("stream %s uses catch-all events.>, which overlaps per-domain streams", stream.Name)
			}
		}
	}
}

// Phase 10 removed the legacy ORDERS/INVENTORY/PRODUCTS streams (and their
// bare order.>, inventory.>, product.> subject patterns). This test guards
// against re-introduction — the per-domain EVENTS_* streams are the single
// source of truth.
func TestDefaultStreamsDoNotIncludeLegacyStreams(t *testing.T) {
	bannedNames := map[string]struct{}{
		"ORDERS":        {},
		"INVENTORY":     {},
		"PRODUCTS":      {},
		"NOTIFICATIONS": {},
	}
	bannedSubjects := map[string]struct{}{
		"order.>":     {},
		"inventory.>": {},
		"product.>":   {},
	}

	for _, stream := range DefaultStreams {
		if _, banned := bannedNames[stream.Name]; banned {
			t.Errorf("legacy stream %s still in DefaultStreams", stream.Name)
		}
		for _, subject := range stream.Subjects {
			if _, banned := bannedSubjects[subject]; banned {
				t.Errorf("stream %s still binds legacy subject %s", stream.Name, subject)
			}
		}
	}
}

func TestDefaultStreamsIncludePerDomainEventStreams(t *testing.T) {
	want := map[string]string{
		"EVENTS_USER":        "events.user.>",
		"EVENTS_ORDER":       "events.order.>",
		"EVENTS_INVENTORY":   "events.inventory.>",
		"EVENTS_CATALOG":     "events.catalog.>",
		"EVENTS_SUPPORT":     "events.support.>",
		"EVENTS_MARKETPLACE": "events.marketplace.>",
	}

	for _, stream := range DefaultStreams {
		if subject, ok := want[stream.Name]; ok {
			if len(stream.Subjects) != 1 || stream.Subjects[0] != subject {
				t.Fatalf("%s subjects = %v, want [%s]", stream.Name, stream.Subjects, subject)
			}
			delete(want, stream.Name)
		}

		if strings.HasPrefix(stream.Name, "EVENTS_") && stream.MaxBytes <= 0 {
			t.Fatalf("%s MaxBytes must be set", stream.Name)
		}
	}

	for name := range want {
		t.Fatalf("missing default stream %s", name)
	}
}
