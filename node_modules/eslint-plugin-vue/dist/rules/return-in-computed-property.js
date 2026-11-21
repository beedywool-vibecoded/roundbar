const require_index = require("../utils/index.js");
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/rules/return-in-computed-property.ts
/**
* @fileoverview Enforces that a return statement is present in computed property (return-in-computed-property)
* @author Armano
*/
var return_in_computed_property_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce that a return statement is present in computed property",
			categories: ["vue3-essential", "vue2-essential"],
			url: "https://eslint.vuejs.org/rules/return-in-computed-property.html"
		},
		fixable: null,
		schema: [{
			type: "object",
			properties: { treatUndefinedAsUnspecified: { type: "boolean" } },
			additionalProperties: false
		}],
		messages: {
			expectedReturnInFunction: "Expected to return a value in computed function.",
			expectedReturnInProperty: "Expected to return a value in \"{{name}}\" computed property."
		}
	},
	create(context) {
		const shouldTreatUndefinedAsUnspecified = (context.options[0] || {}).treatUndefinedAsUnspecified !== false;
		const computedProperties = /* @__PURE__ */ new Set();
		const computedFunctionNodes = [];
		return Object.assign({ Program(program) {
			const tracker = new _eslint_community_eslint_utils.ReferenceTracker(require_index.default.getScope(context, program));
			const map = { computed: { [_eslint_community_eslint_utils.ReferenceTracker.CALL]: true } };
			for (const { node } of require_index.default.iterateReferencesTraceMap(tracker, map)) {
				if (node.type !== "CallExpression") continue;
				const getter = require_index.default.getGetterBodyFromComputedFunction(node);
				if (getter) computedFunctionNodes.push(getter);
			}
		} }, require_index.default.defineVueVisitor(context, { onVueObjectEnter(obj) {
			for (const computedProperty of require_index.default.getComputedProperties(obj)) computedProperties.add(computedProperty);
		} }), require_index.default.executeOnFunctionsWithoutReturn(context, shouldTreatUndefinedAsUnspecified, (node) => {
			for (const cp of computedProperties) if (cp.value && cp.value.parent === node) context.report({
				node,
				messageId: "expectedReturnInProperty",
				data: { name: cp.key || "Unknown" }
			});
			for (const cf of computedFunctionNodes) if (cf === node) context.report({
				node,
				messageId: "expectedReturnInFunction"
			});
		}));
	}
};
exports.default = return_in_computed_property_default;
