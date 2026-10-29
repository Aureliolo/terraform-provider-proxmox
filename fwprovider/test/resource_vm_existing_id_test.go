//go:build acceptance || all

//testacc:tier=medium
//testacc:resource=vm

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package test

import (
	"context"
	"math/rand"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"

	"github.com/bpg/terraform-provider-proxmox/proxmox/nodes/vms"
)

func TestAccResourceVMExistingID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{"vm create", `
			resource "proxmox_virtual_environment_vm" "test" {
				node_name = "{{.NodeName}}"
				vm_id     = {{.ExistingVMID}}
				started   = false
			}`},
		{"vm clone", `
			resource "proxmox_virtual_environment_vm" "test" {
				node_name = "{{.NodeName}}"
				vm_id     = {{.ExistingVMID}}
				started   = false

				clone {
					vm_id   = {{.SourceVMID}}
					retries = 3
				}
			}`},
		{"vm2 create", `
			resource "proxmox_vm" "test" {
				node_name = "{{.NodeName}}"
				id        = {{.ExistingVMID}}
			}`},
		{"cloned vm", `
			resource "proxmox_cloned_vm" "test" {
				node_name = "{{.NodeName}}"
				id        = {{.ExistingVMID}}
				started   = false

				clone = {
					source_vm_id = {{.SourceVMID}}
				}
			}`},
		{"container create", `
			resource "proxmox_virtual_environment_container" "test" {
				node_name      = "{{.NodeName}}"
				vm_id          = {{.ExistingVMID}}
				timeout_create = 60

				operating_system {
					template_file_id = "local:vztmpl/missing.tar.xz"
				}
			}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := InitEnvironment(t)
			ctx := context.Background()

			sourceVMID := 100000 + rand.Intn(99999)
			existingVMID := 100000 + rand.Intn(99999)

			for _, id := range []int{sourceVMID, existingVMID} {
				require.NoError(t, te.NodeClient().VM(0).CreateVM(ctx, &vms.CreateRequestBody{VMID: id}).Err())

				t.Cleanup(func() {
					if result := te.NodeClient().VM(id).DeleteVM(context.Background(), true, true); result.Err() != nil {
						t.Logf("cleanup warning: failed to delete VM %d: %v", id, result.Err())
					}
				})
			}

			existing, err := te.NodeClient().VM(existingVMID).GetVM(ctx)
			require.NoError(t, err)

			// Runs after the post-test destroy, which deletes whatever VM ended up in the state.
			t.Cleanup(func() {
				after, err := te.NodeClient().VM(existingVMID).GetVM(context.Background())
				require.NoError(t, err, "VM %d that already held the ID is gone", existingVMID)
				require.Equal(t, existing, after, "VM %d that already held the ID was changed", existingVMID)
			})

			te.AddTemplateVars(map[string]any{
				"SourceVMID":   sourceVMID,
				"ExistingVMID": existingVMID,
			})

			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: te.AccProviders,
				Steps: []resource.TestStep{{
					Config:      te.RenderConfig(tt.config),
					ExpectError: regexp.MustCompile(`already exists`),
				}},
			})
		})
	}
}
