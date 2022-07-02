# AWS-MFA

Is a very simple utility that allows secure authentication with [multi-factor authentication (MFA) device](https://aws.amazon.com/premiumsupport/knowledge-center/authenticate-mfa-cli/) without storing authentication token in the credential file `~/.aws/credentials`.

# How it works

The environment variables in the AWS CLI have a higher [precedence](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html#cli-configure-quickstart-precedence) than the credentials file, so it is possible to pass these variables only to a child bash process securely without storing the `AWS_SESSION_TOKEN` in the credentials file `~/.aws/credentials`.

So, in practice these environment variables are visible "only" to the bash process that executes the commands, making the authentication process and using the AWS CLI much safer.


## Features
* Pure Go without dependencies *(as it should be)*.
* Retrieves the account id and assigns the `AWS_ACCOUNT` environment value for the child bash process.
* Retrieves the user MFA device Arn automatically.
* Retrieves the session token and stores the `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_SESSION_TOKEN` environment variables that are used by the AWS CLI.
* Automatically closes the bash process when the session token expires.

## Requirements
1. The [MFA-Required policy](aws-mfa-policy.json) **must** be assigned to the user. 

    **Attention:** The two-factor authentication requirement is enforced by the policy, if it is misconfigured, this authentication method will not bring any security improvement.

3. The user must have assigned a MFA device, in the security credentials.
2. [AWS Command Line Interface](https://aws.amazon.com/cli/) must be installed in the local machine.
4. The user account *(only `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`)* must be configured in the local profile `~/.aws/credentials`. 

## MFA-Required policy

The [policy](aws-mfa-policy.json) has the following features

- Denies all resource or service access if MFA is not setup for the IAM User. It only has permission when MFA is not enabled for accessing the IAM User’s page and for adding or deleting MFA.
- This will even deny users with attached AdministratorAccess Policy from accessing other resources if MFA is not enabled.
- User can change password even if MFA is not configured when “User must create a new password at next sign-in” is selected .
- Password change is disabled via IAM Console if the user has not yet configured MFA.
- IAM Users can only see their own IAM settings. They will not be able to see settings for other users.
- IAM User can configure Virtual MFA, U2F and hardware MFA.

The policy above does **NOT** include the following even when MFA is configured.

- Access Keys
- Signing Certificates
- SSH Public Keys for CodeCommit
- Git Credentials for CodeCommit
