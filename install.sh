#!/bin/bash

# install script for bittwiddlers.org

#git pull

pushd stocks-hourly
go install
popd

pushd stocks-web
go install

# clean reinstall templates:
rm -fr /srv/bittwiddlers.org/stocks/static
rm -fr /srv/bittwiddlers.org/stocks/templates
mkdir -p /srv/bittwiddlers.org/stocks/static /srv/bittwiddlers.org/stocks/templates
cp -a static/* /srv/bittwiddlers.org/stocks/static/
cp -a templates/* /srv/bittwiddlers.org/stocks/templates/

sudo cp stocks-web.conf /etc/init/

popd

# restart stocks-web service
sudo stop stocks-web
sudo start stocks-web
