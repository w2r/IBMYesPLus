# Getting started with Go on IBM Cloud
To get started, we'll take you through a sample Go hello world app, help you set up a development environment and deploy to IBM Cloud.

The following instructions are for deploying the application as a Cloud Foundry application. To deploy as a container to **IBM Cloud Kubernetes Service** instead, [see README-kubernetes.md](README-kubernetes.md)

## Prerequisites

You'll need the following:
* [IBM Cloud account](https://console.ng.bluemix.net/registration/)
* [IBM Cloud CLI](https://console.bluemix.net/docs/home/tools)
* [Git](https://git-scm.com/downloads)
* [Go](https://golang.org/dl/)

## 1. Clone the sample app

Now you're ready to start working with the simple Go *hello world* app. Clone the repository and change to the directory where the sample app is located.
  ```
git clone https://github.com/IBM-Cloud/get-started-go
cd get-started-go
  ```

Peruse the files in the *get-started-go* directory to familiarize yourself with the contents.

## 2. Run the app locally

Build and run the app.
  ```
go run get-started-go.go
  ```

View your app at: http://localhost:8080

## 3. Prepare the app for deployment


To deploy to IBM Cloud, it can be helpful to set up a manifest.yml file. One is provided for you with the sample. Take a moment to look at it.

The manifest.yml includes basic information about your app, such as the name, how much memory to allocate for each instance and the route. In this manifest.yml **random-route: true** generates a random route for your app to prevent your route from colliding with others.  You can replace **random-route: true** with **host: myChosenHostName**, supplying a host name of your choice. [Learn more...](https://console.bluemix.net/docs/manageapps/depapps.html#appmanifest)
 ```
 applications:
 - name: GetStartedGo
   random-route: true
   memory: 128M
 ```

## 4. Deploy the app

You can use the IBM Cloud CLI to deploy apps.

  ```
ibmcloud login
ibmcloud target --cf
  ```

From within the *get-started-go* directory push your app to IBM Cloud
  ```
ibmcloud cf push
  ```

This can take a minute. If there is an error in the deployment process you can use the command `ibmcloud cf logs <Your-App-Name> --recent` to troubleshoot.

When deployment completes you should see a message indicating that your app is running.  View your app at the URL listed in the output of the push command.  You can also issue the

 ```
ibmcloud cf apps
 ```
command to view your apps status and see the URL.
