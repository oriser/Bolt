# Creating a Slack app
### Prerequisites
* An external static IP address or DNS leading to Bolt

_If you don't have the listed prerequisites, you can see [how to create it in GCP](./k8s_gcp.md)._

1. Go to [Slack Apps Dashboard](https://api.slack.com/apps)
2. Click `Create New App`
3. Choose `From an app manifest`
4. Select the workspace where you want to deploy the app
5. Copy the content of the [app manifest](../../deploy/app_manifest.yaml) 
and replace each occurrence of `<static_ip>` with the static IP / DNS leading to Bolt.
6. Click `Next` and then `Create`
7. Click `Install To Workspace` and then `Allow` when asking for permissions (If you are not managing the workspace you may need an admin approval)
8. Copy the `Signin Secret` and save aside
9. Got to `Install App` section and copy the `OAuth Token` and save aside
10. Now, you can configure the `SLACK_OAUTH_TOKEN` and `SLACK_SIGNIN_SECRET` for Bolt.
11. After Bolt is running, go to `Event Subscriptions` and click `Retry` for the _Request URL_
12. If all went well, the verification should work, click `Save Changes`
13. Invite Bolt to any channel you want him to join to Wolt food links (use `/add` Slack command)
14. Send a Wolt group link to a channel where Bolt is invited and see it in action :)