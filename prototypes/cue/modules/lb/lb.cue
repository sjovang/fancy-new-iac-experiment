package lb

#Params: {
	name:          string
	location:      string
	resourceGroup: string
	tags:          [string]: string
	publicIpId:    string
}

#Output: {
	name:          "\(name)-lb"
	backendPoolId: "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.Network/loadBalancers/\(name)/backendAddressPools/backendpool"

	resources: {
		lb: {
			type:     "Microsoft.Network/loadBalancers@2023-09-01"
			name:     name
			location: location
			properties: {
				frontendIPConfigurations: [{
					name: "public-frontend"
					properties: publicIPAddress: id: publicIpId
				}]
				backendAddressPools: [{name: "backendpool"}]
				probes: [{
					name: "tcp80"
					properties: {
						protocol:          "Tcp"
						port:              80
						intervalInSeconds: 5
						numberOfProbes:    2
					}
				}]
				loadBalancingRules: [{
					name: "http"
					properties: {
						protocol:             "Tcp"
						frontendPort:         80
						backendPort:          80
						frontendIPConfiguration: id: "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.Network/loadBalancers/\(name)/frontendIPConfigurations/public-frontend"
						backendAddressPool:      id: backendPoolId
						probe:                   id: "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.Network/loadBalancers/\(name)/probes/tcp80"
					}
				}]
			}
		}
	}
}
