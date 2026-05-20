package network

#Params: {
	name:          string
	location:      string
	resourceGroup: string
	tags:          [string]: string
}

#Output: {
	vnetName:          "\(name)-vnet"
	frontendSubnetId:  "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.Network/virtualNetworks/\(vnetName)/subnets/frontend"
	backendSubnetId:   "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.Network/virtualNetworks/\(vnetName)/subnets/backend"
	dataSubnetId:      "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.Network/virtualNetworks/\(vnetName)/subnets/data"
	lbPublicIpId:      "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.Network/publicIPAddresses/\(name)-lb-pip"
	networkSecurityGroupName: "\(name)-nsg"

	resources: {
		vnet: {
			type:     "Microsoft.Network/virtualNetworks@2023-09-01"
			name:     vnetName
			location: location
		}
		frontendSubnet: {
			type: "Microsoft.Network/virtualNetworks/subnets@2023-09-01"
			name: "\(vnetName)/frontend"
		}
		backendSubnet: {
			type: "Microsoft.Network/virtualNetworks/subnets@2023-09-01"
			name: "\(vnetName)/backend"
		}
		dataSubnet: {
			type: "Microsoft.Network/virtualNetworks/subnets@2023-09-01"
			name: "\(vnetName)/data"
		}
		lbPublicIp: {
			type: "Microsoft.Network/publicIPAddresses@2023-09-01"
			name: "\(name)-lb-pip"
		}
		nsg: {
			type: "Microsoft.Network/networkSecurityGroups@2023-09-01"
			name: networkSecurityGroupName
		}
	}
}
