![bolt](docs/assets/bolt_gh.png)

#### Bolt is a Slack bot for Wolt group orders.

### How does it work?
As soon as a Wolt group link is sent to a channel it is part of, 
it joins the group and begins monitoring what each participant has ordered.
Once the order has been purchased, 
a message will be sent indicating how much each participant is required to pay to the group's host (including delivery rate). 
It will even keep reminding the participants to pay until they've marked themselves as paid.

## Features
* Automatic detection of Wolt group links shared to a Slack channel
* Automatic monitoring over participants' ordered items and sending how much each participant has to pay, including delivery rate
* It will try to automatically match the Wolt user to a Slack user and tag the relevant user, in case no matching Slack user found, an admin can add a custom user with `/add-user` command
* Per-order debts reminders 