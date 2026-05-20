package main

input: {
	name:          "expapp-dev"
	location:      "westeurope"
	resourceGroup: "rg-expapp-dev"
	tags: {
		environment: "dev"
		owner:       "platform-team"
	}
	adminUsername: "azureuser"
	vmCount:       2
	vmSize:        "Standard_B2s"
}
