package services

import (
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
)

// GetVersionedGlobalParamsByHeight returns the versioned global params
// for a particular bitcoin height
func (s *Service) GetVersionedGlobalParamsByHeight(height uint64) *types.VersionedGlobalParams {
	// Iterate the list in reverse (i.e. decreasing ActivationHeight)
	// and identify the first element that has an activation height below
	// the specified BTC height.
	for i := len(s.params.Versions) - 1; i >= 0; i-- {
		paramsVersion := s.params.Versions[i]
		if paramsVersion.ActivationHeight <= height {
			return paramsVersion
		}
	}
	return nil
}
