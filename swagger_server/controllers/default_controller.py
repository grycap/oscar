import connexion

from swagger_server.models.delete_function_request import DeleteFunctionRequest
from swagger_server.models.function_definition import FunctionDefinition
from src.providers.openfaas.controller import OpenFaas
from swagger_server.models.in_body import InBody  # noqa: E501


def events_post(body):  # noqa: E501
    """Process Minio events

     # noqa: E501

    :param body: Minio webhook endpoint
    :type body: dict | bytes

    :rtype: None
    """
    return OpenFaas().process_minio_event(body)

def function_async_function_name_post(functionName, data=None):
    """Invoke a function asynchronously

    :param functionName: Function name
    :type functionName: str
    :param data: (Optional) data to pass to function
    :type data: str

    :rtype: None
    """
    return OpenFaas().invoke(functionName, data, asynch=True)


def function_function_name_get(functionName):
    """Get a summary of an OpenFaaS function

    :param functionName: Function name
    :type functionName: str

    :rtype: FunctionListEntry
    """
    return OpenFaas().ls(functionName)


def function_function_name_post(functionName, data=None):
    """Invoke a function defined in OpenFaaS

    :param functionName: Function name
    :type functionName: str
    :param data: (Optional) data to pass to function
    :type data: str

    :rtype: None
    """
    return OpenFaas().invoke(functionName, data)


def functions_delete(body):
    """Remove a deployed function.

    :param body: Function to delete
    :type body: dict | bytes

    :rtype: None
    """
    if connexion.request.is_json:
        body = DeleteFunctionRequest.from_dict(connexion.request.get_json())
    return OpenFaas().rm(body.function_name)


def functions_get():
    """Get a list of deployed functions with: stats and image digest

    :rtype: List[FunctionListEntry]
    """
    return OpenFaas().ls()


def functions_post(body):
    """Deploy a new function.

    :param body: Function to deploy
    :type body: dict | bytes

    :rtype: None
    """
    if connexion.request.is_json:
        params = connexion.request.get_json()
    return OpenFaas().init(**params)


def functions_put(body):
    """Update a function.

    :param body: Function to update
    :type body: dict | bytes

    :rtype: None
    """
    if connexion.request.is_json:
        body = FunctionDefinition.from_dict(connexion.request.get_json())
    return 'do some magic!'
