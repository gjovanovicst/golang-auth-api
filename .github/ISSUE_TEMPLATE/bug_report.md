---
name: Bug Report
about: Create a report to help us improve
labels: bug
---

**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. API endpoint called: `POST /api/your-endpoint`
2. Request payload (if any):
   ```json
   {
     "key": "value"
   }
   ```
3. Request headers (if any):
   ```
   Authorization: Bearer <token>
   ```
4. Relevant environment variables or configuration:
   ```
   DB_HOST=...
   JWT_SECRET=...
   ```
5. Run the request (e.g., with curl, Postman, or frontend)
6. See error/response:
   ```json
   {
     "error": "..."
   }
   ```
7. (Optional) Include relevant logs or stack traces:
   ```
   [2025-06-21 12:00:00] ERROR ...
   ```

**Expected behavior**
A clear and concise description of what you expected to happen.

**Screenshots**
If applicable, add screenshots to help explain your problem.

**Environment (please complete the following information):**
- OS: [e.g. Windows, Mac, Linux]
- Go version: [e.g. 1.22]
- Docker version: [if applicable]

**Additional context**
Add any other context about the problem here.
