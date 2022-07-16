# Create Kubernetes cluster in GCP
GCP has a 300$ free credits when you open an account.
Using the instruction below it will be sufficient to run Bolt for a long period without paying anything.
## Create a project
_If you already have a project you can skip this step_
1. Go to [Google Cloud project selector](https://console.cloud.google.com/projectselector2/home/dashboard) and click on `CREATE PROJECT`
2. Give it a name and click `CREATE`
3. Wait for the project creation process, it will automatically be selected once finished

## Create a Kubernetes cluster
I will demonstrate how to create an _Autopilot_ Kubernetes cluster on GCP.
Autopilot is a fully-managed Kubernetes cluster, which means you don't have to manage the cluster nodes,
and you will pay only for the resources requested (very lean resources required to run Bolt)
1. Go to [Kubernetes Engine](https://console.cloud.google.com/kubernetes/list/overview) page
2. If it's not yet enabled, click `ENABLE` button and wait a few seconds for it to become available
3. Click `CREATE` in order to create a new cluster
4. Choose `CONFIGURE` next to GKE Autopilot
5. Give it a name and region and click `CREATE`
6. It will take a few minutes to create the cluster, in the meantime you can create a static external IP
7. After the cluster has created, click on the cluster name
8. Click on `CONNECT` button, copy and run the provided command.
If you don't have _GCP CLI_ installed, install it from [here](https://cloud.google.com/sdk/docs/install).

## Create a static external IP
Bolt need a static external IP (or DNS) to configure in Slack for incoming events
1. Go to [IP Addresses](https://console.cloud.google.com/networking/addresses/list) page
2. Click on `RESERVE EXTERNAL STATIC ADDRESS`
3. Give it a name, make sure `Preimium` is selected for _Network Service Tier_ and choose a Region (same region as the cluster) and click `RESERVE`
4. Copy the reserved IP address