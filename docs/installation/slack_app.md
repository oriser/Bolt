# Creating a Slack app
### Prerequisites
* An external static IP address or DNS leading to Bolt

_If you don't have the listed prerequisites, you can see [how to create it in GCP](./k8s_gcp.md#create-a-static-external-ip)._

1. Go to [Slack Apps Dashboard](https://api.slack.com/apps)
2. Click `Create New App`
3. Choose [`From an app manifest`](../assets/slack/3_create.png)
4. Select the workspace where you want to deploy the app
5. Copy the content of the [app manifest](../../deploy/app_manifest.yaml) 
and replace each occurrence of `<static_ip>` with the static IP / DNS leading to Bolt.
6. Click `Next` and then `Create`
7. Click `Install To Workspace` and then `Allow` when asking for permissions (If you are not managing the workspace you may need an admin approval)
8. Copy the [`Signin Secret`](../assets/slack/8_creds.png) and save it aside
9. Go to `Install App` section and copy the [`OAuth Token`](../assets/slack/9_token.png) and save it aside
10. You may change the app icon to [Bolt's icon](../assets/bolt_logo_slack.png) - Go to `Basic Information` and find the icon under `Display Information`
11. Now, you can configure `SLACK_OAUTH_TOKEN` and `SLACK_SIGNIN_SECRET` for Bolt and run it. [See here how to run it using Kubernetes](k8s.md)
12. After Bolt is running, go to [`Event Subscriptions`](../assets/slack/11_verify.png) and click `Retry` for the _Request URL_
13. If all went well, the [verification should work](../assets/slack/12_verified.png), click `Save Changes`
14. [Invite Bolt](../assets/slack/13_add.png) to any channel you want him to join to Wolt food links (use `/add` Slack command)
15. Send a Wolt group link to a channel where Bolt is invited and [see it in action](../assets/slack/14_working.png) :)