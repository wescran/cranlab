# CranLab Website
This is the start of my personal website and becoming a member of the IndieWeb

I have started by using the [blank site](https://github.com/indieweb/blank-gh-site) created by IndieWeb to begin my journey!

I am using [Eleventy](https://www.11ty.dev/) as my static site generator.

Website is self-hosted on my own machine. The website is protected through cloudflare using their free tier along with [tunnels](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/)

### NOTE
In order to get eleventy working well on my host system I modified the `fs.inotify.max_user_watches` to be the max value of `524288`
This is based on discussions [here](https://stackoverflow.com/questions/55763428/react-native-error-enospc-system-limit-for-number-of-file-watchers-reached)

The default value is `fs.inotify.max_user_watches = 8192`
