package main

import (
	appmod "iac.experiment/prototypes/cue/modules/app"
	computemod "iac.experiment/prototypes/cue/modules/compute"
	datamod "iac.experiment/prototypes/cue/modules/data"
	identitymod "iac.experiment/prototypes/cue/modules/identity"
	lbmod "iac.experiment/prototypes/cue/modules/lb"
	networkmod "iac.experiment/prototypes/cue/modules/network"
)

// Top-level input (typically selected from environments/*.cue).
input: {
	name:         string
	location:     string
	resourceGroup:string
	tags: [string]: string
	adminUsername:string
	vmCount:      int & >=2 & <=2
	vmSize:       string
}

network: networkmod.#Output & networkmod.#Params & {
	name:         input.name
	location:     input.location
	resourceGroup:input.resourceGroup
	tags:         input.tags
}

identity: identitymod.#Output & identitymod.#Params & {
	name:         input.name
	location:     input.location
	resourceGroup:input.resourceGroup
	tags:         input.tags
}

lb: lbmod.#Output & lbmod.#Params & {
	name:         input.name
	location:     input.location
	resourceGroup:input.resourceGroup
	tags:         input.tags
	publicIpId:   network.lbPublicIpId
}

compute: computemod.#Output & computemod.#Params & {
	name:            input.name
	location:        input.location
	resourceGroup:   input.resourceGroup
	tags:            input.tags
	adminUsername:   input.adminUsername
	vmCount:         input.vmCount
	vmSize:          input.vmSize
	backendPoolId:   lb.backendPoolId
	vmSubnetId:      network.backendSubnetId
	managedIdentityId: identity.id
}

data: datamod.#Output & datamod.#Params & {
	name:              input.name
	location:          input.location
	resourceGroup:     input.resourceGroup
	tags:              input.tags
	dataSubnetId:      network.dataSubnetId
	managedIdentityId: identity.id
}

app: appmod.#Output & appmod.#Params & {
	name:                input.name
	location:            input.location
	resourceGroup:       input.resourceGroup
	tags:                input.tags
	frontendSubnetId:    network.frontendSubnetId
	managedIdentityId:   identity.id
	storageAccountName:  data.storageAccountName
	postgresFqdn:        data.postgresFqdn
	appServicePlanSku:   "P1v3"
}
