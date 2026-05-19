package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEgressOrDefault(t *testing.T) {
	assert.Equal(t, EgressNone, egressOrDefault("", EgressNone))
	assert.Equal(t, EgressOpen, egressOrDefault("", EgressOpen))
	assert.Equal(t, EgressPackageManagers, egressOrDefault(EgressPackageManagers, EgressNone))
}
