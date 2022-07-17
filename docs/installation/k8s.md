# Run using Kubernetes
If you have a Kubernetes cluster, it's very easy to get Bolt up and running.
It doesn't matter where the cluster is running as long as you have the prerequisites and enough permissions on the cluster. 
### Prerequisites
* A Kubernetes cluster and configured `kubectl`
* An external static IP address
* Slack signin secret

_If you don't have a Kubernetes cluster and/or external static IP address, you can see [how to create it in GCP](./k8s_gcp.md)._

Make sure you created a Slack app and you have Slack OAuth Token and Signin Secret. See instructions [here](./slack_app.md).

## Running Bolt
 
### Configuration
You will need at least one Slack user that will be configured as Bolt's admin.
Bolt's admin can map Wolt-users to Slack users if Bolt could not do it itself (using the '/add-user' slash command).

To get the user ID, go to the user's profile, then click on the 3 dots and choose `Copy member ID`
1. Go to [deployment.yaml](../../deploy/deployment.yaml)
2. Paste Slack's OAuth token and signin secret **as Base64 encoded** instead of `<slack_oauth_token_base64>` and `<slack_signin_secret_base64>` respectively
3. Configure `ADMIN_SLACK_USER_IDS` with admin users for Bolt (separated by a comma)
4. Change other configuration as needed in the `ConfigMap` section (see [configuration](../configuration.md) for all available configurations)
5. Replace `<static_ip>` with the reserved static IP 
6. Apply deployment `kubectl apply -f deploy/deployment.yaml`
7. Run `kubectl get pods -n bolt` and make sure bolt is up and running (may take a few minutes)