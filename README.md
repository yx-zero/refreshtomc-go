# Made by yxzero

## 💸 Cheap Minecraft Alts
👉 [https://zzxgp.me/](https://zzxgp.me/)

---

## 🚀 How to use

### edit `config.json`

### how?

**`input_file` is the input file with the refresh tokens to check**

**`proxy_file` is the file with the proxies, in format of ip:port, ONLY HTTP PROXIES ARE ALLOWED**

**`output_file` is the output file where the program would put the extracted mcTokens**

**`concurrent_limit` sets the maximum number of tokens being processed at the same time**.

---

## How does it work?

### in this process:

1. **it gets the microsoft access token using the refresh token**
2. **it gets the xbox token using the microsoft access token**
3. **it gets the xtxs token using the xbox token**
4. **it gets the mcToken using the xtxs token**

every token in each step runs asynchronously, **but each step doesn't**

---

## 📄 Outputs

- ✅ **Success:** `result.txt` by default, changeable in config.json
- ❌ **Failed:** not yet, soon