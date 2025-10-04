// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.
//
// The go-equa library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-equa library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-equa library. If not, see <http://www.gnu.org/licenses/>.

package vm

import "github.com/equa/go-equa/common"

// JumpDestCache represents the cache of jumpdest analysis results.
type JumpDestCache interface {
	// Load retrieves the cached jumpdest analysis for the given code hash.
	// Returns the BitVec and true if found, or nil and false if not cached.
	Load(codeHash common.Hash) (BitVec, bool)

	// Store saves the jumpdest analysis for the given code hash.
	Store(codeHash common.Hash, vec BitVec)
}

// mapJumpDests is the default implementation of JumpDests using a map.
// This implementation is not thread-safe and is meant to be used per EVM instance.
type mapJumpDests map[common.Hash]BitVec

// newMapJumpDests creates a new map-based JumpDests implementation.
func newMapJumpDests() JumpDestCache {
	return make(mapJumpDests)
}

func (j mapJumpDests) Load(codeHash common.Hash) (BitVec, bool) {
	vec, ok := j[codeHash]
	return vec, ok
}

func (j mapJumpDests) Store(codeHash common.Hash, vec BitVec) {
	j[codeHash] = vec
}
