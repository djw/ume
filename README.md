# Ume

Minimal Go rewrite of [awsume](https://awsu.me), but with SSO support

⚠️ Written as a learning exercise, but could be useful for switching between profiles when explicit credentials are required. Most likely, you'll be fine just setting `AWS_PROFILE` and letting the AWS SDKs do the rest. For quick switching I use Oh My Zsh's [aws plugin](https://github.com/ohmyzsh/ohmyzsh/blob/master/plugins/aws/README.md).

## Usage

Make sure both files are you in your path, then create an alias:

```
source ume=". ume-wrapper"
```
