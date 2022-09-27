# TFL status & tracker webappp
* Deployed at [https://tfl.iapps365.com/](https://tfl.iapps365.com/).
* Uses [tfl.gov.uk](https://api.tfl.gov.uk) public APIs to bootstrap and provide status updates.
* Developed as a hobby project in a handful of hours to explore the TFL APIs.
* [Go](https://go.dev) backend with ~zero external dependencies.
* Web front end. Pure HTML and CSS. No javascript.
* Responsive UI thanks to [Bootstrap v5.0](https://getbootstrap.com/docs/5.0/getting-started/introduction/)

# Build & deployment
* Built as a docker image using [Buildpacks](https://buildpacks.io). See Makefile.
* Build using `make build`.
* Binary pushed to [Dockerhub](https://hub.docker.com/r/arunsworld/tfl).
* Binary size: 38MB. All resources including HTML & CSS bundled into the binary using `go:embed`. See main.go.
* Deploy as you would any stateless container.

# TFL APIs used
* [Line APIs](https://api-portal.tfl.gov.uk/api-details#api=Line)
* [Vehicle APIs](https://api-portal.tfl.gov.uk/api-details#api=Vehicle&operation=Vehicle_GetByPathIds)
* For exact APIs used look at tfl-api.go.

# General issues
* TFL real-time updates are very good, but not great. Maybe they can be married with timetable data.
* There doesn't seem to be a straightforward way to track a journey.
* Vehicle tracking is the closest; but it's jumpy and flaky.
* If anyone has better ideas please [drop me a note](mailto:arunsworld@gmail.com) or submit a pull request.