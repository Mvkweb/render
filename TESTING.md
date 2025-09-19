# Testing Plan: Pinterest Scraper Reliability Improvements

This plan outlines the testing strategy for the new reliability features implemented in the `gopin` server.

## 1. Load Testing

*   **Objective:** To ensure the server can handle a high number of concurrent client connections and requests without a significant degradation in performance.
*   **Method:**
    *   Use a tool like `k6` or a custom Go test client to simulate a large number of concurrent WebSocket connections (e.g., 100, 500, 1000).
    *   Each simulated client should connect, request a random number of images, and process the responses.
    *   Monitor server-side metrics: CPU usage, memory usage, and response latency.
*   **Success Criteria:**
    *   The server should maintain a stable and acceptable response time (e.g., < 500ms) under load.
    *   CPU and memory usage should remain within acceptable limits, with no memory leaks.

## 2. Longevity Testing

*   **Objective:** To verify the server's stability and reliability over an extended period.
*   **Method:**
    *   Run the server continuously for 24-48 hours.
    *   During this time, have a smaller number of clients (e.g., 10-20) continuously connect and request images at random intervals.
    *   Monitor the server for memory leaks, crashes, or any other unexpected behavior.
    *   Verify that the background scraping and database cleanup tasks run as expected.
*   **Success Criteria:**
    *   The server should run without any crashes or manual intervention.
    *   Memory usage should remain stable over the entire test period.
    *   The image pool should be regularly refreshed, and old database entries should be cleaned up.

## 3. Failure Testing

*   **Objective:** To test the server's resilience to external failures, such as Pinterest blocking or network issues.
*   **Method:**
    *   **Pinterest Blocking:** Use a tool like `iptables` or a proxy to simulate blocking access to `pinterest.com`.
    *   **Network Latency/Packet Loss:** Use a tool like `tc` to introduce network latency and packet loss between the server and Pinterest.
    *   Observe the behavior of the circuit breaker. It should open when failures are detected and transition to half-open after the timeout.
*   **Success Criteria:**
    *   The circuit breaker should correctly identify failures and prevent further requests to Pinterest.
    *   The server should continue to serve images from the pool even when Pinterest is unavailable.
    *   The server should gracefully recover when Pinterest becomes available again.

## 4. Uniqueness Testing

*   **Objective:** To verify that the duplicate detection system is working correctly and that clients receive unique images.
*   **Method:**
    *   Run multiple clients requesting a large number of images.
    *   Store the hashes of all received images for each client.
    *   Analyze the collected hashes to ensure that there are no duplicates within a single client's session.
*   **Success Criteria:**
    *   Each client should receive a unique set of images for each request.
    *   The database should accurately track the images seen by each client.