package app

#Params: {
	name:               string
	location:           string
	resourceGroup:      string
	tags:               [string]: string
	frontendSubnetId:   string
	managedIdentityId:  string
	storageAccountName: string
	postgresFqdn:       string
	appServicePlanSku:  string
}

#Output: {
	planName: "\(name)-asp"
	appName:  "\(name)-frontend"

	resources: {
		appServicePlan: {
			type:     "Microsoft.Web/serverfarms@2023-01-01"
			name:     planName
			location: location
			kind:     "linux"
			sku: name: appServicePlanSku
			properties: reserved: true
		}
		webApp: {
			type:     "Microsoft.Web/sites@2023-01-01"
			name:     appName
			location: location
			kind:     "app,linux"
			properties: {
				serverFarmId: "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.Web/serverfarms/\(planName)"
				virtualNetworkSubnetId: frontendSubnetId
				siteConfig: {
					linuxFxVersion: "DOTNETCORE|8.0"
					appSettings: [
						{name: "DATABASE_HOST", value: postgresFqdn},
						{name: "STORAGE_ACCOUNT", value: storageAccountName},
					]
				}
			}
			identity: {
				type:                   "UserAssigned"
				userAssignedIdentities: "\(managedIdentityId)": {}
			}
		}
	}
}
