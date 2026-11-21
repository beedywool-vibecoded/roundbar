const require_index = require("../utils/index.js");
const require_casing = require("../utils/casing.js");
const require_regexp = require("../utils/regexp.js");
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/rules/custom-event-name-casing.ts
const ALLOWED_CASE_OPTIONS = ["kebab-case", "camelCase"];
const DEFAULT_CASE = "camelCase";
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
var custom_event_name_casing_default = {
	meta: {
		type: "suggestion",
		docs: {
			description: "enforce specific casing for custom event name",
			categories: void 0,
			url: "https://eslint.vuejs.org/rules/custom-event-name-casing.html"
		},
		fixable: null,
		schema: [{ enum: ALLOWED_CASE_OPTIONS }, {
			type: "object",
			properties: { ignores: {
				type: "array",
				items: { type: "string" },
				uniqueItems: true
			} },
			additionalProperties: false
		}],
		messages: { unexpected: "Custom event name '{{name}}' must be {{caseType}}." }
	},
	create(context) {
		const setupContexts = /* @__PURE__ */ new Map();
		let emitParamName = "";
		const caseType = context.options[0] || DEFAULT_CASE;
		const objectOption = context.options[1] || {};
		const caseChecker = require_casing.getChecker(caseType);
		const isIgnored = require_regexp.toRegExpGroupMatcher(objectOption.ignores);
		/**
		* Check whether the given event name is valid.
		*/
		function isValidEventName(name) {
			return name.split(":").every(caseChecker);
		}
		function verify(nameWithLoc) {
			const name = nameWithLoc.name;
			if (isValidEventName(name) || isIgnored(name)) return;
			context.report({
				loc: nameWithLoc.loc,
				messageId: "unexpected",
				data: {
					name,
					caseType
				}
			});
		}
		const programNode = context.sourceCode.ast;
		const callVisitor = { CallExpression(node, info) {
			const nameWithLoc = require_index.default.getNameParamNode(node);
			if (!nameWithLoc) return;
			const setupContext = setupContexts.get(info ? info.node : programNode);
			if (!setupContext) return;
			const { contextReferenceIds, emitReferenceIds } = setupContext;
			if (node.callee.type === "Identifier" && emitReferenceIds.has(node.callee)) verify(nameWithLoc);
			else {
				const emit = getCalleeMemberNode(node);
				if (emit && emit.name === "emit" && emit.member.object.type === "Identifier" && contextReferenceIds.has(emit.member.object)) verify(nameWithLoc);
			}
		} };
		return require_index.default.defineTemplateBodyVisitor(context, { CallExpression(node) {
			const callee = node.callee;
			const nameWithLoc = require_index.default.getNameParamNode(node);
			if (!nameWithLoc) return;
			if (callee.type === "Identifier" && (callee.name === "$emit" || callee.name === emitParamName)) verify(nameWithLoc);
		} }, require_index.default.compositingVisitors(require_index.default.defineScriptSetupVisitor(context, {
			onDefineEmitsEnter(node) {
				if (!node.parent || node.parent.type !== "VariableDeclarator" || node.parent.init !== node) return;
				const emitParam = node.parent.id;
				if (emitParam.type !== "Identifier") return;
				emitParamName = emitParam.name;
				const variable = (0, _eslint_community_eslint_utils.findVariable)(require_index.default.getScope(context, emitParam), emitParam);
				if (!variable) return;
				const emitReferenceIds = /* @__PURE__ */ new Set();
				for (const reference of variable.references) emitReferenceIds.add(reference.identifier);
				setupContexts.set(programNode, {
					contextReferenceIds: /* @__PURE__ */ new Set(),
					emitReferenceIds
				});
			},
			...callVisitor
		}), require_index.default.defineVueVisitor(context, {
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
			...callVisitor,
			onVueObjectExit(node) {
				setupContexts.delete(node);
			}
		}), { CallExpression(node) {
			const nameLiteralNode = require_index.default.getNameParamNode(node);
			if (!nameLiteralNode) return;
			const emit = getCalleeMemberNode(node);
			if (emit && emit.name === "$emit") verify(nameLiteralNode);
		} }));
	}
};
exports.default = custom_event_name_casing_default;
