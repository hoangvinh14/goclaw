import type { ChatGPTOAuthRoutingConfig } from "@/types/agent";
import {
  normalizeChatGPTOAuthRoutingInput,
  type NormalizedChatGPTOAuthRouting,
} from "./agent-display-utils";

export function buildDraftRouting(
  savedRouting: NormalizedChatGPTOAuthRouting,
  hasProviderDefaults: boolean,
): ChatGPTOAuthRoutingConfig {
  if (savedRouting.isExplicit) {
    return {
      override_mode: savedRouting.overrideMode,
      strategy: savedRouting.strategy,
      extra_provider_names: savedRouting.extraProviderNames,
    };
  }

  if (hasProviderDefaults) {
    return {
      override_mode: "inherit",
      strategy: "primary_first",
      extra_provider_names: [],
    };
  }

  return {
    override_mode: "custom",
    strategy: "primary_first",
    extra_provider_names: [],
  };
}

export function routingDraftSignature(
  routing: ChatGPTOAuthRoutingConfig,
  hasProviderDefaults: boolean,
): string {
  const normalized = normalizeChatGPTOAuthRoutingInput(routing);
  if (normalized.overrideMode === "inherit" && hasProviderDefaults) {
    return JSON.stringify({ override_mode: "inherit" });
  }
  return JSON.stringify({
    override_mode: "custom",
    strategy: normalized.strategy,
    extra_provider_names: normalized.extraProviderNames,
  });
}
