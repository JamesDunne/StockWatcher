#!/bin/bash

# install script for bittwiddlers.org

#git pull

echo Building stocks-hourly...
pushd stocks-hourly
go install
cp $GOPATH/bin/stocks-hourly /srv/bittwiddlers.org/stocks/
cp emails.tmpl /srv/bittwiddlers.org/stocks/
popd

echo Building stocks-web...
pushd stocks-web
go install

# clean reinstall templates:
rm -fr /srv/bittwiddlers.org/stocks/static
rm -fr /srv/bittwiddlers.org/stocks/templates
mkdir -p /srv/bittwiddlers.org/stocks/static /srv/bittwiddlers.org/stocks/templates
cp -a static/* /srv/bittwiddlers.org/stocks/static/
cp -a templates/* /srv/bittwiddlers.org/stocks/templates/

sudo cp stocks-web.conf /etc/init/
sudo stop stocks-web
cp $GOPATH/bin/stocks-web /srv/bittwiddlers.org/stocks/
sudo start stocks-web
popd
