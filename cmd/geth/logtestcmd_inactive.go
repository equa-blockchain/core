// Copyright 2023 The go-equa Authors
// This file is part of go-equa.
//
// go-equa is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-equa is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-equa. If not, see <http://www.gnu.org/licenses/>.

//go:build !integrationtests

package main

import "github.com/urfave/cli/v2"

var logTestCommand *cli.Command
