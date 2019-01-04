#!/bin/bash

./notary init notary.rsc.io
./notary sign rsc.io/quote@v1.5.2
./notary sign rsc.io/quote@v1.0.0
./notary log
