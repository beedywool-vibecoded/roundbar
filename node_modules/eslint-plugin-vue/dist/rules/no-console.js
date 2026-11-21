const require_index = require("../utils/index.js");
// lib/rules/no-console.ts
/**
* @author ItMaga <https://github.com/ItMaga>
* See LICENSE file in root directory for full license.
*/
var no_console_default = require_index.default.wrapCoreRule("no-console", {
	skipBaseHandlers: true,
	create(context) {
		const allowed = (context.options[0] || {}).allow || [];
		/**
		* Copied from the core rule `no-console`.
		* Checks whether the property name of the given MemberExpression node
		* is allowed by options or not.
		*/
		function isAllowed(node) {
			const propertyName = require_index.default.getStaticPropertyName(node);
			return propertyName && allowed.includes(propertyName);
		}
		return { MemberExpression(node) {
			if (node.object.type === "Identifier" && node.object.name === "console" && !isAllowed(node)) context.report({
				node: node.object,
				loc: node.object.loc,
				messageId: "unexpected"
			});
		} };
	}
});
exports.default = no_console_default;
