const require_index = require("../utils/index.js");
// lib/rules/no-this-in-before-route-enter.ts
/**
* @fileoverview Don't use this in a beforeRouteEnter method
* @author Przemyslaw Jan Beigert
*/
var no_this_in_before_route_enter_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow `this` usage in a `beforeRouteEnter` method",
			categories: null,
			url: "https://eslint.vuejs.org/rules/no-this-in-before-route-enter.html"
		},
		fixable: null,
		schema: [],
		messages: { disallow: "'beforeRouteEnter' does NOT have access to `this` component instance. https://router.vuejs.org/guide/advanced/navigation-guards.html#in-component-guards." }
	},
	create(context) {
		const beforeRouteEnterFunctions = /* @__PURE__ */ new Set();
		let scopeStack = null;
		function onFunctionEnter(node) {
			if (node.type === "ArrowFunctionExpression") return;
			scopeStack = {
				upper: scopeStack,
				node,
				beforeRouteEnter: beforeRouteEnterFunctions.has(node)
			};
		}
		function onFunctionExit(node) {
			if (scopeStack && scopeStack.node === node) scopeStack = scopeStack.upper;
		}
		return require_index.default.defineVueVisitor(context, {
			onVueObjectEnter(node) {
				const beforeRouteEnter = require_index.default.findProperty(node, "beforeRouteEnter");
				if (beforeRouteEnter && beforeRouteEnter.value.type === "FunctionExpression") beforeRouteEnterFunctions.add(beforeRouteEnter.value);
			},
			":function": onFunctionEnter,
			":function:exit": onFunctionExit,
			ThisExpression(node) {
				if (scopeStack && scopeStack.beforeRouteEnter) context.report({
					node,
					messageId: "disallow"
				});
			}
		});
	}
};
exports.default = no_this_in_before_route_enter_default;
