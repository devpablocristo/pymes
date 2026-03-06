from __future__ import annotations

from collections.abc import AsyncIterator
from typing import Any

from google import genai
from google.genai import types

from src.llm.base import ChatChunk, Message, ToolDeclaration


class GeminiProvider:
    def __init__(self, api_key: str, model: str = "gemini-2.0-flash") -> None:
        self.client = genai.Client(api_key=api_key)
        self.model = model

    async def chat(
        self,
        messages: list[Message],
        tools: list[ToolDeclaration] | None = None,
        temperature: float = 0.3,
        max_tokens: int = 2048,
    ) -> AsyncIterator[ChatChunk]:
        system_parts = [m.content for m in messages if m.role == "system"]
        system_instruction = "\n\n".join(system_parts) if system_parts else None

        config = types.GenerateContentConfig(
            temperature=temperature,
            max_output_tokens=max_tokens,
            system_instruction=system_instruction,
        )
        if tools:
            config.tools = [types.Tool(function_declarations=[self._to_gemini_tool(t) for t in tools])]

        non_system = [m for m in messages if m.role != "system"]

        response = self.client.models.generate_content_stream(
            model=self.model,
            contents=self._to_gemini_messages(non_system),
            config=config,
        )

        for chunk in response:
            if not chunk.candidates:
                continue
            candidate = chunk.candidates[0]
            if not candidate.content or not candidate.content.parts:
                continue
            for part in candidate.content.parts:
                if getattr(part, "function_call", None):
                    function_call = part.function_call
                    yield ChatChunk(
                        type="tool_call",
                        tool_call={
                            "name": function_call.name,
                            "arguments": dict(function_call.args or {}),
                        },
                    )
                elif getattr(part, "text", None):
                    yield ChatChunk(type="text", text=part.text)

        yield ChatChunk(type="done")

    def _to_gemini_messages(self, messages: list[Message]) -> list[types.Content]:
        converted: list[types.Content] = []
        for msg in messages:
            if msg.role == "assistant":
                parts: list[types.Part] = []
                if msg.content:
                    parts.append(types.Part.from_text(text=msg.content))
                if msg.tool_calls:
                    for tc in msg.tool_calls:
                        parts.append(types.Part.from_function_call(
                            name=tc.get("name", ""),
                            args=tc.get("arguments", {}),
                        ))
                if parts:
                    converted.append(types.Content(role="model", parts=parts))
            elif msg.role == "tool":
                converted.append(types.Content(
                    role="user",
                    parts=[types.Part.from_function_response(
                        name=msg.tool_call_id or "unknown",
                        response={"result": msg.content},
                    )],
                ))
            else:
                converted.append(types.Content(
                    role="user",
                    parts=[types.Part.from_text(text=msg.content or "")],
                ))
        return converted

    def _to_gemini_tool(self, tool: ToolDeclaration) -> types.FunctionDeclaration:
        return types.FunctionDeclaration(
            name=tool.name,
            description=tool.description,
            parameters=self._to_schema(tool.parameters),
        )

    def _to_schema(self, schema: dict[str, Any]) -> types.Schema:
        schema_type = str(schema.get("type", "object")).upper()
        properties = schema.get("properties", {})
        required = schema.get("required", [])
        converted_properties: dict[str, types.Schema] = {}
        for key, value in properties.items():
            prop_type = str(value.get("type", "string")).upper()
            if prop_type == "ARRAY":
                items_schema = value.get("items", {"type": "object"})
                converted_properties[key] = types.Schema(
                    type="ARRAY",
                    description=value.get("description"),
                    items=self._to_schema(items_schema),
                )
            elif prop_type == "OBJECT" and value.get("properties"):
                converted_properties[key] = self._to_schema(value)
            else:
                converted_properties[key] = types.Schema(
                    type=prop_type,
                    description=value.get("description"),
                )
        return types.Schema(type=schema_type, properties=converted_properties, required=required)
