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
