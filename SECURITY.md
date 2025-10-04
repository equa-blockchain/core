# Security Policy

## 🔒 Reporting a Vulnerability

**CRITICAL:** Do NOT report security vulnerabilities through public GitHub issues.

### Report Via

**Email:** security@equa.network

**PGP Key:** [Download](https://equa.network/security.asc)

**Include:**
- Type of vulnerability
- Full paths of affected files
- Location of affected code (tag/branch/commit)
- Step-by-step reproduction
- Proof-of-concept or exploit code (if possible)
- Impact assessment

### Response Timeline

- **24 hours:** Acknowledgment of report
- **72 hours:** Initial assessment and severity classification
- **7 days:** Mitigation plan
- **30 days:** Fix released (critical vulnerabilities)

### Disclosure Policy

- Security issues will be patched before public disclosure
- Reporter will be credited (unless they prefer anonymity)
- Public disclosure 90 days after fix release

## 🏆 Bug Bounty Program

**Coming soon:** We're planning a bug bounty program with rewards for:
- Critical: $5,000 - $25,000
- High: $1,000 - $5,000
- Medium: $500 - $1,000
- Low: $100 - $500

## 🛡️ Security Best Practices

### For Node Operators

1. **Keep Updated:** Always run latest stable version
2. **Firewall:** Only expose necessary ports
3. **SSH:** Use key-based auth, disable password
4. **Monitoring:** Set up alerts for unusual activity
5. **Backups:** Regular backups of keystore and data

### For Developers

1. **GPG Sign:** All commits must be signed
2. **Dependencies:** Regularly update and audit
3. **Code Review:** Minimum 2 reviewers for sensitive code
4. **Testing:** Comprehensive security tests
5. **Secrets:** Never commit private keys, passwords

## 📜 Security Audits

### Completed

None yet (project in development)

### Planned

- **Q2 2025:** Smart contract audit (Trail of Bits)
- **Q3 2025:** Consensus mechanism audit (OpenZeppelin)
- **Q4 2025:** Full system penetration test

## 🔐 Cryptographic Standards

- **Hashing:** Keccak-256 (SHA-3)
- **Signatures:** ECDSA (secp256k1), BLS12-381
- **Encryption:** AES-256-GCM
- **Key Derivation:** PBKDF2, scrypt

## ✅ Security Checklist

Before each release:

- [ ] All dependencies updated
- [ ] Security scan passed (gosec, staticcheck)
- [ ] No secrets in code
- [ ] Signed commits only
- [ ] Audit logs reviewed
- [ ] Penetration test completed (major releases)


**Last Updated:** 2025-01-15
