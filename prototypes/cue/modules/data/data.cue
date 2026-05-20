package data

import "strings"

#Params: {
	name:              string
	location:          string
	resourceGroup:     string
	tags:              [string]: string
	dataSubnetId:      string
	managedIdentityId: string
}

#Output: {
	postgresServerName: "\(name)-pg"
	postgresDbName:     "appdb"
	postgresFqdn:       "\(postgresServerName).postgres.database.azure.com"
	storageAccountName: strings.Replace(strings.ToLower("\(name)st"), "-", "", -1)

	resources: {
		postgresServer: {
			type:     "Microsoft.DBforPostgreSQL/flexibleServers@2023-06-01-preview"
			name:     postgresServerName
			location: location
			properties: {
				version: "16"
				delegatedSubnetResourceId: dataSubnetId
				network: publicNetworkAccess: "Disabled"
			}
			identity: {
				type:                   "UserAssigned"
				userAssignedIdentities: "\(managedIdentityId)": {}
			}
		}
		postgresDb: {
			type: "Microsoft.DBforPostgreSQL/flexibleServers/databases@2023-06-01-preview"
			name: "\(postgresServerName)/\(postgresDbName)"
		}
		storageAccount: {
			type:     "Microsoft.Storage/storageAccounts@2023-05-01"
			name:     storageAccountName
			location: location
			kind:     "StorageV2"
			sku:      name: "Standard_LRS"
		}
	}
}
