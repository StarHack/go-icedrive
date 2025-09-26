# go-icedrive

My simple approach to implementing the Icedrive API.

**Status: WiP, this will be transformed into a library later on**

Currently supports:

- Login
  - Username/password incl. proof-of-work solution (captcha)
  - Bearer token
- List Folder
- Upload File (unencrypted)
- Download File (unencrypted)
- Move File / Folder to trash
- Empty Trash

## Getting Started

Copy `.env-sample` to `.env` and use your own email + password. You may then change `main.go` to use the commented out login block for a username/password login.