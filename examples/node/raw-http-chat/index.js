const httpBaseUrl = process.env.ERION_EMBER_HTTP_URL || "http://localhost:8080";
const prompt = "How do I explain HTTP cache misses to a teammate?";
const ttlSeconds = 3600;

async function main() {
  const ready = await requestJSON("GET", "/ready");
  if (ready.status !== "ready") {
    throw new Error(`service not ready: status=${JSON.stringify(ready.status)}`);
  }

  const firstLookup = await requestJSON("POST", "/v1/cache/get", {
    prompt,
    similarity_threshold: 0.85,
  });
  printLookup("first lookup", firstLookup);

  if (!firstLookup.hit) {
    const llmResponse = generateLLMResponse(prompt);
    const setResponse = await requestJSON("POST", "/v1/cache/set", {
      prompt,
      response: llmResponse,
      ttl: ttlSeconds,
    });

    console.log(`cache miss -> placeholder LLM response stored with id=${setResponse.id}`);
  }

  const secondLookup = await requestJSON("POST", "/v1/cache/get", {
    prompt,
    similarity_threshold: 0.85,
  });
  printLookup("second lookup", secondLookup);

  const stats = await requestJSON("GET", "/v1/stats");
  console.log(
    `stats: hits=${stats.cache_hits} misses=${stats.cache_misses} total_queries=${stats.total_queries}`,
  );
}

async function requestJSON(method, route, body) {
  const response = await fetch(`${httpBaseUrl}${route}`, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (!response.ok) {
    throw new Error(`${method} ${route} failed: status=${response.status} body=${JSON.stringify(await response.text())}`);
  }

  return response.json();
}

function generateLLMResponse(inputPrompt) {
  return `[placeholder LLM] A simple answer for "${inputPrompt}" is: a cache miss means the requested response was not found yet, so your app should call the upstream model and then store the new answer.`;
}

function printLookup(label, response) {
  if (response.hit) {
    console.log(
      `${label}: hit=true exact_match=${response.exact_match} similarity=${Number(response.similarity).toFixed(2)} response=${JSON.stringify(response.response)}`,
    );
    return;
  }

  console.log(`${label}: hit=false -> call your LLM and then cache the answer`);
}

main().catch((error) => {
  console.error(`example failed: ${error.message}`);
  process.exitCode = 1;
});
