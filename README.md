# funceasy-cli
Command Line Tools For FuncEasy Deploy and Resource Generation

## Install the CLI

```
export RELEASE=(${the version})
export OS=$(uname -s| tr '[:upper:]' '[:lower:]')
curl -OL https://github.com/FuncEasy/funceasy-cli/releases/download/$RELEASE/funceasy-cli-$OS-amd64.zip && \
unzip funceasy-cli-$OS-amd64.zip && \
sudo mv ./funceasy-cli /usr/local/bin/
```

## Usage

```
funceasy-cli --help
```
