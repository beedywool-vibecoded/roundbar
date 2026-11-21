const require_index = require("../utils/index.js");
// lib/rules/no-deprecated-events-api.ts
/**
* @fileoverview disallow using deprecated events api
* @author yoyo930021
*/
var no_deprecated_events_api_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow using deprecated events api (in Vue.js 3.0.0+)",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/no-deprecated-events-api.html"
		},
		fixable: null,
		schema: [],
		messages: { noDeprecatedEventsApi: "The Events api `$on`, `$off` `$once` is deprecated. Using external library instead, for example mitt." }
	},
	create(context) {
		return require_index.default.defineVueVisitor(context, { "CallExpression > MemberExpression, CallExpression > ChainExpression > MemberExpression"(node) {
			const call = node.parent.type === "ChainExpression" ? node.parent.parent : node.parent;
			if (call.optional || require_index.default.skipChainExpression(call.callee) !== node || ![
				"$on",
				"$off",
				"$once"
			].includes(require_index.default.getStaticPropertyName(node) || "") || !require_index.default.isThis(node.object, context)) return;
			context.report({
				node: node.property,
				messageId: "noDeprecatedEventsApi"
			});
		} });
	}
};
exports.default = no_deprecated_events_api_default;
