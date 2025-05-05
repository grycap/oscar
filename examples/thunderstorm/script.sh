#!/bin/sh

CONTAINER_GENERATED_OUTPUT_DIR="/srv/outputs/Prediction_5min"

python3 -m ai4eosc_thunder_nowcast_ml.models.api predict \
    --select_dtm_pr server_13AREAs_neighbors_binary_Zlin_Meri_5min_pr \
    --select_mlo_pr server_mlflow_settings_5min_pr \
    --select_nnw_pr server_13AREAs_neighbors_neural_network_binary_pr \
    --select_ino_pr server_pr_lastdataFalse_lastmodelFalse_NN2_5min \
    --select_usr_pr server_petersisan_MicroStepMIS

cp -R "$CONTAINER_GENERATED_OUTPUT_DIR"/* "$TMP_OUTPUT_DIR"/

exit $?