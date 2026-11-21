const require_index = require("../utils/index.js");
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/rules/require-slots-as-functions.ts
/**
* @author Yosuke Ota
* See LICENSE file in root directory for full license.
*/
const SLOT_USED_AS_VALUE_PARENT_TYPES = /* @__PURE__ */ new Set([
	"MemberExpression",
	"VariableDeclarator",
	"SpreadElement",
	"ArrayExpression"
]);
var require_slots_as_functions_default = {
	meta: {
		type: "problem",
		docs: {
			description: "enforce properties of `$slots` to be used as a function",
			categories: ["vue3-essential"],
			url: "https://eslint.vuejs.org/rules/require-slots-as-functions.html"
		},
		fixable: null,
		schema: [],
		messages: { unexpected: "Property in `$slots` should be used as function." }
	},
	create(context) {
		/**
		* Verify the given node
		* @param node The node to verify
		* @param reportNode The node to report
		*/
		function verify(node, reportNode) {
			const parent = node.parent;
			if (parent.type === "VariableDeclarator" && parent.id.type === "Identifier") {
				verifyReferences(parent.id, reportNode);
				return;
			}
			if (parent.type === "AssignmentExpression" && parent.right === node && parent.left.type === "Identifier") {
				verifyReferences(parent.left, reportNode);
				return;
			}
			if (parent.type === "ChainExpression") {
				verify(parent, reportNode);
				return;
			}
			if (SLOT_USED_AS_VALUE_PARENT_TYPES.has(parent.type)) context.report({
				node: reportNode,
				messageId: "unexpected"
			});
		}
		/**
		* Verify the references of the given node.
		* @param node The node to verify
		* @param reportNode The node to report
		*/
		function verifyReferences(node, reportNode) {
			const variable = (0, _eslint_community_eslint_utils.findVariable)(require_index.default.getScope(context, node), node);
			if (!variable) return;
			for (const reference of variable.references) {
				if (!reference.isRead()) continue;
				const id = reference.identifier;
				verify(id, reportNode);
			}
		}
		return require_index.default.defineVueVisitor(context, { MemberExpression(node) {
			const object = require_index.default.skipChainExpression(node.object);
			if (object.type !== "MemberExpression" || require_index.default.getStaticPropertyName(object) !== "$slots" || !require_index.default.isThis(object.object, context) || node.property.type === "PrivateIdentifier") return;
			verify(node, node.property);
		} });
	}
};
exports.default = require_slots_as_functions_default;
