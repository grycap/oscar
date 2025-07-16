#!/bin/sh

nginx && exec su mlserver -c "mlserver start /opt/mlserver/models"