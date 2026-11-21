const require_index = require("../utils/index.js");
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/rules/no-side-effects-in-computed-properties.ts
/**
* @fileoverview Don't introduce side effects in computed properties
* @author Michał Sajnóg
*/
var no_side_effects_in_computed_properties_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow side effects in computed properties",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/no-side-effects-in-computed-properties.html"
		},
		fixable: null,
		schema: [],
		messages: {
			unexpectedSideEffectInFunction: "Unexpected side effect in computed function.",
			unexpectedSideEffectInProperty: "Unexpected side effect in \"{{key}}\" computed property."
		}
	},
	create(context) {
		const computedPropertiesMap = /* @__PURE__ */ new Map();
		const computedCallNodes = [];
		const setupRanges = [];
		let scopeStack = null;
		function onFunctionEnter(node) {
			scopeStack = {
				upper: scopeStack,
				body: node.body
			};
		}
		function onFunctionExit() {
			scopeStack &&= scopeStack.upper;
		}
		const nodeVisitor = {
			":function": onFunctionEnter,
			":function:exit": onFunctionExit,
			"MemberExpression > :matches(Identifier, ThisExpression)"(node, info) {
				if (!scopeStack) return;
				const targetBody = scopeStack.body;
				const computedProperty = (info ? computedPropertiesMap.get(info.node) || [] : []).find((cp) => cp.value && require_index.default.isInRange(cp.value, node) && targetBody === cp.value);
				if (computedProperty) {
					const mem = node.parent;
					if (mem.object !== node) return;
					const isThis = require_index.default.isThis(node, context);
					const isVue = node.type === "Identifier" && node.name === "Vue";
					const invalid = mem.parent.type === "CallExpression" && mem.property.type === "Identifier" && (isThis && mem.property.name === "$set" || isVue && mem.property.name === "set") ? { node: mem.property } : isThis && require_index.default.findMutating(mem);
					if (invalid) context.report({
						node: invalid.node,
						messageId: "unexpectedSideEffectInProperty",
						data: { key: computedProperty.key || "Unknown" }
					});
					return;
				}
				if (node.type === "ThisExpression") return;
				const computedFunction = computedCallNodes.find((c) => require_index.default.isInRange(c, node) && targetBody === c.body);
				if (!computedFunction) return;
				if (node.parent.object !== node) return;
				const variable = (0, _eslint_community_eslint_utils.findVariable)(require_index.default.getScope(context, node), node);
				if (!variable || variable.defs.length !== 1) return;
				const def = variable.defs[0];
				if ([
					"ImplicitGlobalVariable",
					"TDZ",
					"ImportBinding"
				].includes(def.type)) return;
				if (!setupRanges.some(([start, end]) => require_index.default.isInRange([start, end], def.node)) || require_index.default.isInRange(computedFunction, def.node)) return;
				const invalid = require_index.default.findMutating(node);
				if (invalid) context.report({
					node: invalid.node,
					messageId: "unexpectedSideEffectInFunction"
				});
			}
		};
		const scriptSetupNode = require_index.default.getScriptSetupElement(context);
		if (scriptSetupNode) setupRanges.push(scriptSetupNode.range);
		return require_index.default.compositingVisitors({ Program(program) {
			const tracker = new _eslint_community_eslint_utils.ReferenceTracker(require_index.default.getScope(context, program));
			const computedRefs = require_index.default.iterateReferencesTraceMap(tracker, { computed: { [_eslint_community_eslint_utils.ReferenceTracker.CALL]: true } });
			for (const { node } of computedRefs) {
				if (node.type !== "CallExpression") continue;
				const getterBody = require_index.default.getGetterBodyFromComputedFunction(node);
				if (getterBody) computedCallNodes.push(getterBody);
			}
		} }, scriptSetupNode ? require_index.default.defineScriptSetupVisitor(context, nodeVisitor) : require_index.default.defineVueVisitor(context, {
			onVueObjectEnter(node) {
				computedPropertiesMap.set(node, require_index.default.getComputedProperties(node));
			},
			onSetupFunctionEnter(node) {
				setupRanges.push(node.body.range);
			},
			...nodeVisitor
		}));
	}
};
exports.default = no_side_effects_in_computed_properties_default;
