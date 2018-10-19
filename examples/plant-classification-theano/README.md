# Plant Classification with Lasagne/Theano

Example cloned from: https://github.com/indigo-dc/plant-classification-theano

To run this example you need:

1. An elastic cluster running OSCAR
2. The container to deploy available in docker hub:
    * You can create your own with:
      ```
      docker build -t {docker-hub-user}/{docker-hub-image} .
      docker push {docker-hub-user}/{docker-hub-image}
      ```
    * Or use the container already available in docker hub: 
      ```
      grycap/oscar-theano-plants
      ```

3. The script to execute coded in base64. You can use the python script available in the examples folder:
```
python ../tobase64.py < script.sh
```
  
4. Then, to create the function you have two options:
    * Use the OSCAR interface available at the IP where you deployed your cluster in the port `32112`
  ```
  {
  "image": "grycap/oscar-theano-plants",
  "name": "plant-classifier",
  "script": "IyEvYmluL2Jhc2gKCmVjaG8gIlNDUklQVDogSW52b2tlZCBjbGFzc2lmeV9pbWFnZS5weS4gRmlsZSBhdmFpbGFibGUgaW4gJFNDQVJfSU5QVVRfRklMRSIKRklMRV9OQU1FPWBiYXNlbmFtZSAkU0NBUl9JTlBVVF9GSUxFYApPVVRQVVRfRklMRT0kU0NBUl9PVVRQVVRfRk9MREVSLyRGSUxFX05BTUUKCnB5dGhvbjIgL29wdC9wbGFudC1jbGFzc2lmaWNhdGlvbi10aGVhbm8vY2xhc3NpZnlfaW1hZ2UucHkgJFNDQVJfSU5QVVRfRklMRSAtbyAkT1VUUFVUX0ZJTEU="
  }
  ```
    * Use curl to do a POST request:
  ```
  curl -X POST --header 'Content-Type: application/json' --header 'Accept: text/plain' -d '{ \ 
  "image": "grycap/oscar-theano-plants", \ 
  "name": "plant-classifier", \ 
  "script": "IyEvYmluL2Jhc2gKCmVjaG8gIlNDUklQVDogSW52b2tlZCBjbGFzc2lmeV9pbWFnZS5weS4gRmlsZSBhdmFpbGFibGUgaW4gJFNDQVJfSU5QVVRfRklMRSIKRklMRV9OQU1FPWBiYXNlbmFtZSAkU0NBUl9JTlBVVF9GSUxFYApPVVRQVVRfRklMRT0kU0NBUl9PVVRQVVRfRk9MREVSLyRGSUxFX05BTUUKCnB5dGhvbjIgL29wdC9wbGFudC1jbGFzc2lmaWNhdGlvbi10aGVhbm8vY2xhc3NpZnlfaW1hZ2UucHkgJFNDQVJfSU5QVVRfRklMRSAtbyAkT1VUUFVUX0ZJTEU=" \ 
  }' 'http://${OSCAR_ENDPOINT}:32112/functions'
  ```
  Once OSCAR finishes the creation of the function, the corresponding buckets in minio are also created.

5. To execute the function that will process the image you need to upload the image to process to the corresponding `plant-classifier-in`
