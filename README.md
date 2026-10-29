# ItslearningDL.go

This Application downloads all file resources that a student has access to from the [Itslearning](https://itslearning.com/global/) learning platform, and stores them on the local disk, retaining the original file structure.

## Setup

To build this project into an executable binary, make sure you have Golang installed and run

```
go build
```

Then create a `.env` file in the same directory as the binary and set your itslearning credentials

```
# Your itslearning username
ITSLEARNING_USERNAME=
# Your itslearning password
ITSLEARNING_PASSWORD=
# The itslearning instance to use
# ex. https://berlin.itslearning.com
ITSLEARNING_INSTANCE=
```

And run the applcation by executing the binary

```
./itslearningdl
```

If everything was successful, the program will now create an `out` folder and save all downloaded files into it.

## SSO login

If your school logs in through SSO, leave `ITSLEARNING_USERNAME` and `ITSLEARNING_PASSWORD` empty (or pass `-sso`).
The login page opens in your browser. After you sign in, the browser asks to open "Itslearning Login". Click Open and the download continues in the terminal.
On macOS a small helper app for the `itsl-itslearning://` link is installed in `~/Library/Application Support/itslearningdl/`. On Windows the program registers itself (per user, no admin needed) as the handler for that link, so the browser hands the login straight back to it. On other systems, paste the redirect URL into the terminal.
The login is saved in `.itslearning-token.json`, so you only need to sign in again when it expires.
