const path = require("node:path");

const grpc = require("@grpc/grpc-js");
const protoLoader = require("@grpc/proto-loader");

const grpcAddress = process.env.ERION_EMBER_GRPC_ADDR || "localhost:9090";
const prompt = "How do I explain Go channels to a teammate?";
const ttlSeconds = 3600;

const protoPath = path.resolve(__dirname, "../../../proto/ember/v1/semantic_cache.proto");
const packageDefinition = protoLoader.loadSync(protoPath, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});
const proto = grpc.loadPackageDefinition(packageDefinition).ember.v1;

async function main() {
  const client = new proto.SemanticCacheService(
    grpcAddress,
    grpc.credentials.createInsecure(),
  );

  try {
    await unary(client, "Health", {});

    const firstLookup = await unary(client, "Get", {
      prompt,
      similarity_threshold: 0.85,
    });
    printLookup("first lookup", firstLookup);

    if (!firstLookup.hit) {
      const llmResponse = generateLLMResponse(prompt);
      const setResponse = await unary(client, "Set", {
        prompt,
        response: llmResponse,
        ttl_seconds: String(ttlSeconds),
      });

      console.log(`cache miss -> placeholder LLM response stored with id=${setResponse.id}`);
    }

    const secondLookup = await unary(client, "Get", {
      prompt,
      similarity_threshold: 0.85,
    });
    printLookup("second lookup", secondLookup);

    const stats = await unary(client, "Stats", {});
    console.log(
      `stats: hits=${stats.cache_hits} misses=${stats.cache_misses} total_queries=${stats.total_queries}`,
    );
  } finally {
    client.close();
  }
}

function unary(client, method, request) {
  return new Promise((resolve, reject) => {
    client[method](request, (err, response) => {
      if (err) {
        reject(err);
        return;
      }

      resolve(response);
    });
  });
}

function generateLLMResponse(inputPrompt) {
  return `[placeholder LLM] A simple answer for "${inputPrompt}" is: channels let goroutines synchronize and pass work safely.`;
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
