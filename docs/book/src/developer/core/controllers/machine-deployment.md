# MachineDeployment

A MachineDeployment orchestrates deployments over a fleet of MachineSets.

Its main responsibilities are:
* Adopting matching MachineSets not assigned to a MachineDeployment
* Adopting matching MachineSets not assigned to a Cluster
* Managing the Machine deployment process
  * Scaling up new MachineSets when changes are made
  * Scaling down old MachineSets when newer MachineSets replace them
* Updating the status of MachineDeployment objects

![](../../../images/cluster-admission-machinedeployment-controller.png)

## In-place propagation
Changes to the following fields of the MachineDeployment are propagated in-place to the MachineSet and do not trigger a full rollout:
- `.annotations`
- `.spec.deletion.order`
- `.spec.template.metadata.labels`
- `.spec.template.metadata.annotations`
- `.spec.template.spec.minReadySeconds`
- `.spec.template.spec.deletion.nodeDrainTimeout`
- `.spec.template.spec.deletion.nodeDeletionTimeout`
- `.spec.template.spec.deletion.nodeVolumeDetachTimeout`

Note: In cases where changes to any of these fields are paired with rollout causing changes, the new values are propagated only to the new MachineSet. 
