package identity

#Params: {
	name:          string
	location:      string
	resourceGroup: string
	tags:          [string]: string
}

#Output: {
	name:        "\(name)-uami"
	id:          "/subscriptions/<sub>/resourceGroups/\(resourceGroup)/providers/Microsoft.ManagedIdentity/userAssignedIdentities/\(name)"
	principalId: "<resolved-at-runtime>"

	resource: {
		type:     "Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31"
		name:     name
		location: location
	}

	roleAssignments: [
		{
			scope: "/subscriptions/<sub>/resourceGroups/\(resourceGroup)"
			role:  "Storage Blob Data Contributor"
		},
		{
			scope: "/subscriptions/<sub>/resourceGroups/\(resourceGroup)"
			role:  "Contributor"
		},
	]
}
