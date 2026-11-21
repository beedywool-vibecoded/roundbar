const require_dom_events = require("../utils/dom-events.json.js");
const require_index = require("../utils/index.js");
let _eslint_community_eslint_utils = require("@eslint-community/eslint-utils");
// lib/rules/no-shadow-native-events.ts
/**
* Check if the given name matches defineEmitsNode variable name
*/
function isEmitVariableName(name, defineEmitsNode) {
	const node = defineEmitsNode?.parent;
	return node?.type === "VariableDeclarator" && node.id.type === "Identifier" && name === node.id.name;
}
var no_shadow_native_events_default = {
	meta: {
		type: "problem",
		docs: {
			description: "disallow the use of event names that collide with native web event names",
			url: "https://eslint.vuejs.org/rules/no-shadow-native-events.html",
			categories: void 0
		},
		schema: [],
		messages: { violation: "Avoid shadowing the native event \"{{ name }}\"." }
	},
	create(context) {
		const setupContexts = /* @__PURE__ */ new Map();
		let vueTemplateDefineData = null;
		const programNode = context.sourceCode.ast;
		if (require_index.default.isScriptSetup(context)) vueTemplateDefineData = {
			type: "setup",
			define: programNode
		};
		/**
		* Verify if an emit call violates the rule of not using a native dom event name.
		*/
		function verifyEmit(nameWithLoc) {
			const name = nameWithLoc.name.toLowerCase();
			if (!require_dom_events.default.includes(name)) return;
			context.report({
				loc: nameWithLoc.loc,
				messageId: "violation",
				data: { name: nameWithLoc.name }
			});
		}
		/**
		* Verify if an emit declaration violates the rule of not using a native dom event name.
		*/
		const verifyEmitDeclaration = (emits) => {
			for (const { node, emitName } of emits) {
				if (!node || !emitName || !require_dom_events.default.includes(emitName.toLowerCase())) continue;
				context.report({
					messageId: "violation",
					data: { name: emitName },
					loc: node.loc
				});
			}
		};
		const callVisitor = { CallExpression(node, info) {
			const callee = require_index.default.skipChainExpression(node.callee);
			const nameWithLoc = require_index.default.getNameParamNode(node);
			if (!nameWithLoc) return;
			const vueDefineNode = info ? info.node : programNode;
			let emit;
			if (callee.type === "MemberExpression") {
				const name = require_index.default.getStaticPropertyName(callee);
				if (name === "emit" || name === "$emit") emit = {
					name,
					member: callee
				};
			}
			const setupContext = setupContexts.get(vueDefineNode);
			if (setupContext) {
				const { contextReferenceIds, emitReferenceIds } = setupContext;
				if (callee.type === "Identifier" && emitReferenceIds.has(callee)) verifyEmit(nameWithLoc);
				else if (emit?.name === "emit") {
					const memObject = require_index.default.skipChainExpression(emit.member.object);
					if (memObject.type === "Identifier" && contextReferenceIds.has(memObject)) verifyEmit(nameWithLoc);
				}
			}
			if (emit?.name !== "$emit") return;
			const memObject = require_index.default.skipChainExpression(emit.member.object);
			if (require_index.default.isThis(memObject, context)) verifyEmit(nameWithLoc);
		} };
		const scriptSetupVisitor = require_index.default.defineScriptSetupVisitor(context, {
			onDefineEmitsEnter: (node, emits) => {
				verifyEmitDeclaration(emits);
				if (vueTemplateDefineData?.type === "setup") vueTemplateDefineData.defineEmits = node;
				if (node.parent?.type !== "VariableDeclarator" || node.parent.init !== node) return;
				const emitParam = node.parent.id;
				const variable = emitParam.type === "Identifier" ? (0, _eslint_community_eslint_utils.findVariable)(require_index.default.getScope(context, emitParam), emitParam) : null;
				if (!variable) return;
				const emitReferenceIds = /* @__PURE__ */ new Set();
				for (const reference of variable.references) {
					if (!reference.isRead()) continue;
					emitReferenceIds.add(reference.identifier);
				}
				setupContexts.set(programNode, {
					contextReferenceIds: /* @__PURE__ */ new Set(),
					emitReferenceIds
				});
			},
			...callVisitor
		});
		const vueVisitor = require_index.default.defineVueVisitor(context, {
			onSetupFunctionEnter(node, { node: vueNode }) {
				const contextParam = node.params[1];
				if (!contextParam || contextParam.type === "RestElement" || contextParam.type === "ArrayPattern") return;
				const contextReferenceIds = /* @__PURE__ */ new Set();
				const emitReferenceIds = /* @__PURE__ */ new Set();
				if (contextParam.type === "ObjectPattern") {
					const emitProperty = require_index.default.findAssignmentProperty(contextParam, "emit");
					if (!emitProperty) return;
					const emitParam = emitProperty.value;
					const variable = emitParam.type === "Identifier" ? (0, _eslint_community_eslint_utils.findVariable)(require_index.default.getScope(context, emitParam), emitParam) : null;
					if (!variable) return;
					for (const reference of variable.references) {
						if (!reference.isRead()) continue;
						emitReferenceIds.add(reference.identifier);
					}
				} else if (contextParam.type === "Identifier") {
					const variable = (0, _eslint_community_eslint_utils.findVariable)(require_index.default.getScope(context, contextParam), contextParam);
					if (!variable) return;
					for (const reference of variable.references) {
						if (!reference.isRead()) continue;
						contextReferenceIds.add(reference.identifier);
					}
				}
				setupContexts.set(vueNode, {
					contextReferenceIds,
					emitReferenceIds
				});
			},
			onVueObjectEnter(node) {
				const emits = require_index.default.getComponentEmitsFromOptions(node);
				verifyEmitDeclaration(emits);
			},
			onVueObjectExit(node, { type }) {
				if ((!vueTemplateDefineData || vueTemplateDefineData.type !== "export" && vueTemplateDefineData.type !== "setup") && [
					"mark",
					"export",
					"definition"
				].includes(type)) vueTemplateDefineData = {
					type,
					define: node
				};
				setupContexts.delete(node);
			},
			...callVisitor
		});
		return require_index.default.compositingVisitors(require_index.default.defineTemplateBodyVisitor(context, { CallExpression(node) {
			const callee = require_index.default.skipChainExpression(node.callee);
			const nameWithLoc = require_index.default.getNameParamNode(node);
			if (!nameWithLoc) return;
			if (callee.type === "Identifier" && (callee.name === "$emit" || vueTemplateDefineData?.defineEmits && isEmitVariableName(callee.name, vueTemplateDefineData.defineEmits))) verifyEmit(nameWithLoc);
		} }, require_index.default.compositingVisitors(scriptSetupVisitor, vueVisitor)));
	}
};
exports.default = no_shadow_native_events_default;
