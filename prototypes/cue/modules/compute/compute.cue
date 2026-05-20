package compute

#Params: {
	name:              string
	location:          string
	resourceGroup:     string
	tags:              [string]: string
	adminUsername:     string
	vmCount:           int & >=2 & <=2
	vmSize:            string
	vmSubnetId:        string
	backendPoolId:     string
	managedIdentityId: string
}

#Output: {
	availabilitySetName: "\(name)-aset"
	nicNames: [for i in [0, 1] {"\(name)-vm\(i)-nic"}]
	vmNames:  [for i in [0, 1] {"\(name)-vm\(i)"}]

	resources: {
		availabilitySet: {
			type:     "Microsoft.Compute/availabilitySets@2023-09-01"
			name:     availabilitySetName
			location: location
		}
		nics: [for i in [0, 1] {
			type: "Microsoft.Network/networkInterfaces@2023-09-01"
			name: "\(name)-vm\(i)-nic"
			properties: {
				ipConfigurations: [{
					name: "ipconfig1"
					properties: {
						subnet: id: vmSubnetId
						loadBalancerBackendAddressPools: [{id: backendPoolId}]
					}
				}]
			}
		}]
		virtualMachines: [for i in [0, 1] {
			type:     "Microsoft.Compute/virtualMachines@2023-09-01"
			name:     "\(name)-vm\(i)"
			location: location
			properties: {
				availabilitySet: id: "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.Compute/availabilitySets/\(availabilitySetName)"
				hardwareProfile:  vmSize: vmSize
				osProfile: {
					computerName:  "\(name)-vm\(i)"
					adminUsername: adminUsername
				}
				networkProfile: {
					networkInterfaces: [{
						id: "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.Network/networkInterfaces/\(name)-vm\(i)-nic"
					}]
				}
				storageProfile: imageReference: {
					publisher: "Canonical"
					offer:     "0001-com-ubuntu-server-jammy"
					sku:       "22_04-lts-gen2"
					version:   "latest"
				}
			}
			identity: {
				type:                   "UserAssigned"
				userAssignedIdentities: "\(managedIdentityId)": {}
			}
		}]
	}
}
