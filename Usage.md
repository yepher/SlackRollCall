# Slack Roll Call

Many folks are moving to Slack to help build community. Often times there is a desire to welcome new members to help them more easily acclimate with the community. SlackRollCall helps to solve that problem by easily listing out new members and list handles that are no longer part of the community.

Another use case is you are a remote worker and it is hard to keep track of folks that join or leave the company. Slack is usually a good indication of that.


## Usage

The easiest way to use Slack Roll call is to setup your teams [Slack Auth Token](https://api.slack.com/docs/oauth-test-tokens) as an environment variable. The variable should be called `SLACK_API_KEY`. You can pass it from the command line too with the `-k YOUR_KEY`. This might be helpful if you are tracking multiple Slack communities. If you are tracking multiple communities you will also want to keep each one in it's one cache file with `--cache [CACHE FILE NAME]`.


The first time SlackRollCall is run it will create a cache of current users. Each time after that the current Slack _user list_ will be compared to the existing list. If you want SlackRollCall to update the cache after it reports changes you need to pass this command line switch `-u "true"`.




```
./SlackRollCall --help
NAME:
   Slack Role Call - Slack Role Call

USAGE:
   SlackRollCall [global options] command [command options] [arguments...]
   
VERSION:
   0.0.1
   
COMMANDS:
   help, h	Shows a list of commands or help for one command
   
GLOBAL OPTIONS:
   --apikey, -k 			Required Slack API key [$SLACK_API_KEY]
   --verbose "false"			Dumps additional information to console
   , -c "./userList.cache"	Set cache file to use.
   --updatecache, -u "false"		Saves all current members to cache
   --help, -h				show help
   --version, -v			print the version
```


**Sample Output**

```
Searching for MIA
	--- Member, Bobby Watson, bobby.watson@example.com, isDelete: YES
Searching for new members
	+++ New Member, John Doe, john.doe@example.com - 
	+++ New Member, Jane Doe, jane.doe@example.com - 
```

## Setup

SlackRollCall uses this the [User.List](https://api.slack.com/methods/users.list) command. In order for that command to work it needs a user [User Auth Token](https://api.slack.com/docs/oauth-test-tokens).


## Next Steps

* Enable SlackRollCall to send a daily email with a membership change list using
* Post change list to a private Slack channel



