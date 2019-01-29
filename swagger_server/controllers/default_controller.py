import connexion

from swagger_server.models.function_definition import FunctionDefinition
from src.providers.onpremises.controller import OnPremises

def events_post(body):
    """Process Minio events

     # noqa: E501

    :param body: Minio webhook endpoint
    :type body: dict | bytes

    :rtype: None
    """
    return OnPremises().process_minio_event(body)

def function_function_name_get(functionName):
    """Get a summary of an OpenFaaS function

    :param functionName: Function name
    :type functionName: str

    :rtype: FunctionListEntry
    """
    params = {'name' : functionName}
    return OnPremises(params).ls()

def functions_get():
    """Get a list of deployed functions with: stats and image digest

    :rtype: List[FunctionListEntry]
    """
    return OnPremises().ls()

def function_function_name_post(functionName, data=None):
    """Invoke a function defined in OpenFaaS

    :param functionName: Function name
    :type functionName: str
    :param data: (Optional) data to pass to function
    :type data: str

    :rtype: None
    """
    params = {'name' : functionName}    
    return OnPremises(params).invoke(data)

def function_async_function_name_post(functionName, data=None):
    """Invoke a function asynchronously

    :param functionName: Function name
    :type functionName: str
    :param data: (Optional) data to pass to function
    :type data: str

    :rtype: None
    """
    params = {'name' : functionName}      
    return OnPremises(params).invoke(data, asynch=True)

def functions_delete(body):
    """Remove a deployed function.

    :param body: Function to delete
    :type body: dict | bytes

    :rtype: None
    """
    if connexion.request.is_json:
        params = connexion.request.get_json()
    return OnPremises(params).rm()

def functions_post(body):
    """Deploy a new function.

    :param body: Function to deploy
    :type body: dict | bytes

    :rtype: None
    """
    if connexion.request.is_json:
        params = connexion.request.get_json()
    return OnPremises(params).init()


def functions_put(body):
    """Update a function.

    :param body: Function to update
    :type body: dict | bytes

    :rtype: None
    """
    if connexion.request.is_json:
        params = connexion.request.get_json()
    return OnPremises(params).update()
