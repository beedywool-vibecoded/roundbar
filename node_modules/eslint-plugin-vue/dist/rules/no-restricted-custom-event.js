const require_index = require("../utils/index.js");
const require_regexp = require("../utils/regexp.js");
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/rules/no-restricted-custom-event.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
function parseOption(option) {
	if (typeof option === "string") {
		const matcher = require_regexp.toRegExp(option, { remove: "g" });
		return { test(name) {
			return matcher.test(name);
		} };
	}
	const parsed = parseOption(option.event);
	parsed.message = option.message;
	parsed.suggest = option.suggest;
	return parsed;
}
/**
* Get the callee member node from the given CallExpression
*/
function getCalleeMemberNode(node) {
	const callee = require_index.default.skipChainExpression(node.callee);
	if (callee.type === "MemberExpression") {
		const name = require_index.default.getStaticPropertyName(callee);
		if (name) return {
			name,
			member: callee
		};
	}
	return null;
}
var no_restricted_custom_event_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "disallow specific custom event",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/no-restricted-custom-event.html"
		},
		fixable: null,
		hasSuggestions: true,
		schema: {
			type: "array",
			items: { oneOf: [{ type: ["string"] }, {
				type: "object",
				properties: {
					event: { type: "string" },
					message: {
						type: "string",
						minLength: 1
					},
					suggest: { type: "string" }
				},
				required: ["event"],
				additionalProperties: false
			}] },
			uniqueItems: true,
			minItems: 0
		},
		messages: {
			restrictedEvent: "{{message}}",
			instead: "Instead, change to `{{suggest}}`."
		}
	},
	create(context) {
		const setupContexts = /* @__PURE__ */ new Map();
		const options = context.options.map(parseOption);
		function verify(nameWithLoc) {
			const name = nameWithLoc.name;
			for (const option of options) if (option.test(name)) {
				const message = option.message || `Using \`${name}\` event is not allowed.`;
				context.report({
					loc: nameWithLoc.loc,
					messageId: "restrictedEvent",
					data: { message },
					suggest: option.suggest ? [{
						fix(fixer) {
							const sourceCode = context.sourceCode;
							return fixer.replaceTextRange(nameWithLoc.range, `${sourceCode.text[nameWithLoc.range[0]]}${JSON.stringify(option.suggest).slice(1, -1).replaceAll("'", String.raw`\'`)}${sourceCode.text[nameWithLoc.range[1] - 1]}`);
						},
						messageId: "instead",
						data: { suggest: option.suggest }
					}] : []
				});
				break;
			}
		}
		return require_index.default.defineTemplateBodyVisitor(context, { CallExpression(node) {
			const callee = node.callee;
			const nameWithLoc = require_index.default.getNameParamNode(node);
			if (!nameWithLoc) return;
			if (callee.type === "Identifier" && callee.name === "$emit") verify(nameWithLoc);
		} }, require_index.default.compositingVisitors(require_index.default.defineVueVisitor(context, {
			onSetupFunctionEnter(node, { node: vueNode }) {
				const contextParam = require_index.default.skipDefaultParamValue(node.params[1]);
				if (!contextParam || contextParam.type === "RestElement" || contextParam.type === "ArrayPattern") return;
				const contextReferenceIds = /* @__PURE__ */ new Set();
				const emitReferenceIds = /* @__PURE__ */ new Set();
				if (contextParam.type === "ObjectPattern") {
					const emitProperty = require_index.default.findAssignmentProperty(contextParam, "emit");
					if (!emitProperty || emitProperty.value.type !== "Identifier") return;
					const emitParam = emitProperty.value;
					const variable = (0, _eslint_community_eslint_utils.findVariable)(require_index.default.getScope(context, emitParam), emitParam);
					if (!variable) return;
					for (const reference of variable.references) emitReferenceIds.add(reference.identifier);
				} else {
					const variable = (0, _eslint_community_eslint_utils.findVariable)(require_index.default.getScope(context, contextParam), contextParam);
					if (!variable) return;
					for (const reference of variable.references) contextReferenceIds.add(reference.identifier);
				}
				setupContexts.set(vueNode, {
					contextReferenceIds,
					emitReferenceIds
				});
			},
			CallExpression(node, { node: vueNode }) {
				const nameWithLoc = require_index.default.getNameParamNode(node);
				if (!nameWithLoc) return;
				const setupContext = setupContexts.get(vueNode);
				if (!setupContext) return;
				const { contextReferenceIds, emitReferenceIds } = setupContext;
				if (node.callee.type === "Identifier" && emitReferenceIds.has(node.callee)) verify(nameWithLoc);
				else {
					const emit = getCalleeMemberNode(node);
					if (emit && emit.name === "emit" && emit.member.object.type === "Identifier" && contextReferenceIds.has(emit.member.object)) verify(nameWithLoc);
				}
			},
			onVueObjectExit(node) {
				setupContexts.delete(node);
			}
		}), { CallExpression(node) {
			const nameWithLoc = require_index.default.getNameParamNode(node);
			if (!nameWithLoc) return;
			const emit = getCalleeMemberNode(node);
			if (emit && emit.name === "$emit") verify(nameWithLoc);
		} }));
	}
};
exports.default = no_restricted_custom_event_default;
