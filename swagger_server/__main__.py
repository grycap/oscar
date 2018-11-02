#!/usr/bin/env python3

import connexion
import sys
sys.path.append(".")
sys.path.append("..")
from swagger_server import encoder

def main():
    app = connexion.App(__name__, specification_dir='./swagger/')
    app.app.json_encoder = encoder.JSONEncoder
    app.add_api('swagger.yaml', arguments={'title': 'On-premises Serverless Container-aware ARchitectures API Gateway'})
    app.run(port=8080, debug=True)


if __name__ == '__main__':
    main()
