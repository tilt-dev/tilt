# Docker Compose Support (Alpha)
Good news, everyone: you can now run Tilt with your existing Docker Compose config! No need to write new boilerplate, or cobble together Kubernetes YAML before you're actually running on Kubernetes. Just follow the instructions below to get started.

## Up and Running In Two Lines
To run your Docker Compose setup via Tilt, all you need is a simple `Tiltfile` that tells Tilt where to find your config:

```
echo 'docker_compose("/path/to/docker-compose.yml")' > Tiltfile
tilt up
```

And that's it! You should see your services spinning up in Tilt, with status, errors, and logs all easily visible thanks to our Heads-Up Display.

## What We Mean By "Alpha"
So far, we've only built bare-bones Docker Compose support, but we think even the functionality we have so far can improve your life a bit, and we're building more features every day! 

You might want to try the current support if:
* you've been curious about Tilt, and you don't have k8s YAML at the ready, but you DO have a Docker Compose setup
* you're frustrated with Docker Compose's log barf and want to easily find logs and errors per-service
* you're excited about dev tools and trying new things
* you enjoy filing bug reports and helping make software better

It's probably not for you if:
* you're looking for a polished product to pitch to the rest of your team _today_

Additionally, there are certain use cases that the current iteration of Tilt x Docker Compose is more or less suited for:

### What to expect from Tilt x Docker Compose, by your use case

* You use Docker Compose for your entire development flow: it handles both building Docker images and spinning them up in the appropriate containers 

Congrats, we think that Tilt x Docker Compose as it exists today will be great for you! If you edit code locally, Tilt will automatically rebuild and redeploy the appropriate service, plus you get all the visibility of our Heads-Up Display. 

* You use Docker Compose to spin up images that have been built elsewhere (e.g. you have to run `make build` before you run `docker-compose up`) 

You won't get the benefits of automatic rebuild, but you'll still have vastly better visibility into your app: you can easily get logs per-service, see status at a glance, and find crashes as soon as they happen.

Don't worry, we're working on a way for you to get the benefits of auto-rebuilding (with some caching magic to make your builds lightning fast); but in the meantime, we think you'll still get something out of running your Docker Compose setup via Tilt.  

* You build Docker images via Docker Compose (i.e. you specify `build` in your config file) AND you make use of `MOUNT` / `VOLUME` in your `Dockerfile` or `docker-compose.yml`

We're still working out the kinks for this use case. We hope it'll work fine, but depending on the specifics of your situation, Tilt may try to kick off a rebuild when you change a `MOUNT`ed file -- which is obviously silly, since that file automatically gets updated in the container. You have been warned.

* Your containers automatically restart after crashes (i.e. you set [container restart policy](https://docs.docker.com/compose/compose-file/#restart) in your config file).

Most of our functionality will still work, but we're still working on how to surface the right error messages to you when something goes wrong. When your containers crash and restart, you might see some odd stuff in the display. We're working on tightening up this experience.


### How to be an alpha user
We're pushing new stuff every day, so don't bother waiting for releases; install from `master`!

We want your feedback! If you see weird behavior, if we don't quiiite support your use case, or there are features that would make you fall in love with Tilt, [file an issue](https://github.com/windmilleng/tilt/issues) and let us know!
