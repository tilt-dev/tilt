# GoDotEnv

A Go (golang) port of the Ruby dotenv project (which loads env vars from a .env file)

From the original Library:

> Storing configuration in the environment is one of the tenets of a twelve-factor app. Anything that is likely to change between deployment environments–such as resource handles for databases or credentials for external services–should be extracted from the code into environment variables.
>
> But it is not always practical to set environment variables on development machines or continuous integration servers where multiple projects are run. Dotenv load variables from a .env file into ENV when the environment is bootstrapped.

This is a fork of [joho/godotenv](https://github.com/joho/godotenv) focussing on `.env` file support by the compose specification



To run linter and tests, please install [Earthly](https://earthly.dev/get-earthly) and run:
```sh
earthly +all
```
