# Overview

## Docs
[Withlings API docs](https://developer.withings.com/developer-guide/v3/integration-guide/public-health-data-api/developer-account/create-your-accesses-no-medical-cloud)

## Inital Planning Tasks
- [ ] Read guide
  - [x] Create application and dev account
  - [x] get secrets
  - [ ] Understand the OAuth Flow and create document explaining the steps involved

## Projects

### Expose to internet

- [x] Public accessability
  - [x] Create tailscale tunnel 
  - [x] Provide the tunnel to withlings application

Start funnel
```bash
tailscale funnel --https=443 --bg http://localhost:8080
```

Stop funnel
```bash
tailscale funnel --https=443 off
```

### OAuth

[Example](https://github.com/withings-sas/api-oauth2-python)

- [ ] Redirect user to Authorisation URL
  - [ ] Expose login endpoint
  - [ ] Generate Authorisation URL 
        `https://account.withings.com/oauth2_user/authorize2?response_type=code&client_id=YOUR_CLIENT_ID&scope=user.info,user.metrics,user.activity&redirect_uri=YOUR_REDIRECT_URI&state=YOUR_STATE`
  - [ ] Redirect user
- [ ] User gets redirected to callback URL provided in auth url
  - [ ] parse parameters `code` & `state`
  - [ ] validate state
- [ ] Exchange authorisation code for access token and refresh token
- [ ] Ask for authorization to get user information
- [ ] Pull some inital data
- [ ] Auto refresh token

### Get Data

- [ ] Setup webhook for data retreval
- [ ] Persist data into pg storeage

### Retreval service 

- [ ] New service (only accessable via tailscale)
- [ ] Store latest info from pg storage
- [ ] Get endpoint to serve data

### Create Shortcut

- [ ] Get data from retreval endpoint
- [ ] Turn into a dictionary
- [ ] Persist data into health

### CI

- [ ] Jenkins pipeline to deploy services