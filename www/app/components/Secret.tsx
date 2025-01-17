/// <reference path="../References.d.ts"/>
import * as React from 'react';
import * as SecretTypes from '../types/SecretTypes';
import * as OrganizationTypes from '../types/OrganizationTypes';
import * as SecretActions from '../actions/SecretActions';
import * as MiscUtils from '../utils/MiscUtils';
import PageInput from './PageInput';
import PageSelect from './PageSelect';
import PageInfo from './PageInfo';
import PageTextArea from './PageTextArea';
import PageSave from './PageSave';
import ConfirmButton from './ConfirmButton';
import Help from './Help';
import * as Constants from "../Constants";

interface Props {
	secret: SecretTypes.SecretRo;
	organizations: OrganizationTypes.OrganizationsRo;
}

interface State {
	disabled: boolean;
	changed: boolean;
	message: string;
	secret: SecretTypes.Secret;
}

const css = {
	card: {
		position: 'relative',
		padding: '10px 10px 0 10px',
		marginBottom: '5px',
	} as React.CSSProperties,
	remove: {
		position: 'absolute',
		top: '5px',
		right: '5px',
	} as React.CSSProperties,
	domain: {
		margin: '9px 5px 0 5px',
		height: '20px',
	} as React.CSSProperties,
	itemsLabel: {
		display: 'block',
	} as React.CSSProperties,
	itemsAdd: {
		margin: '8px 0 15px 0',
	} as React.CSSProperties,
	group: {
		flex: 1,
		minWidth: '280px',
		margin: '0 10px',
	} as React.CSSProperties,
	save: {
		paddingBottom: '10px',
	} as React.CSSProperties,
	label: {
		width: '100%',
		maxWidth: '280px',
	} as React.CSSProperties,
	inputGroup: {
		width: '100%',
	} as React.CSSProperties,
};

export default class Secret extends React.Component<Props, State> {
	constructor(props: any, context: any) {
		super(props, context);
		this.state = {
			disabled: false,
			changed: false,
			message: '',
			secret: null,
		};
	}

	set(name: string, val: any): void {
		let secret: any;

		if (this.state.changed) {
			secret = {
				...this.state.secret,
			};
		} else {
			secret = {
				...this.props.secret,
			};
		}

		secret[name] = val;

		this.setState({
			...this.state,
			changed: true,
			secret: secret,
		});
	}

	onSave = (): void => {
		this.setState({
			...this.state,
			disabled: true,
		});
		SecretActions.commit(this.state.secret).then((): void => {
			this.setState({
				...this.state,
				message: 'Your changes have been saved',
				changed: false,
				disabled: false,
			});

			setTimeout((): void => {
				if (!this.state.changed) {
					this.setState({
						...this.state,
						message: '',
						changed: false,
						secret: null,
					});
				}
			}, 3000);
		}).catch((): void => {
			this.setState({
				...this.state,
				message: '',
				disabled: false,
			});
		});
	}

	onDelete = (): void => {
		this.setState({
			...this.state,
			disabled: true,
		});
		SecretActions.remove(this.props.secret.id).then((): void => {
			this.setState({
				...this.state,
				disabled: false,
			});
		}).catch((): void => {
			this.setState({
				...this.state,
				disabled: false,
			});
		});
	}

	render(): JSX.Element {
		let secr: SecretTypes.Secret = this.state.secret ||
			this.props.secret;

		let organizationsSelect: JSX.Element[] = [];
		organizationsSelect.push(
			<option key="null" value="">
				Node Secret
			</option>,
		);
		if (this.props.organizations.length) {
			for (let organization of this.props.organizations) {
				organizationsSelect.push(
					<option
						key={organization.id}
						value={organization.id}
					>{organization.name}</option>,
				);
			}
		}

		return <div
			className="bp3-card"
			style={css.card}
		>
			<div className="layout horizontal wrap">
				<div style={css.group}>
					<div style={css.remove}>
						<ConfirmButton
							safe={true}
							className="bp3-minimal bp3-intent-danger bp3-icon-trash"
							progressClassName="bp3-intent-danger"
							dialogClassName="bp3-intent-danger bp3-icon-delete"
							dialogLabel="Delete Secret"
							confirmMsg="Permanently delete this secret"
							confirmInput={true}
							items={[secr.name]}
							disabled={this.state.disabled}
							onConfirm={this.onDelete}
						/>
					</div>
					<PageInput
						label="Name"
						help="Name of secret"
						type="text"
						placeholder="Name"
						value={secr.name}
						onChange={(val): void => {
							this.set('name', val);
						}}
					/>
					<PageTextArea
						label="Comment"
						help="Secret comment."
						placeholder="Secret comment"
						rows={3}
						value={secr.comment}
						onChange={(val: string): void => {
							this.set('comment', val);
						}}
					/>
					<PageInput
						label="AWS Key ID"
						help="AWS key ID."
						type="text"
						placeholder="Key ID"
						value={secr.key}
						onChange={(val: string): void => {
							this.set('key', val);
						}}
					/>
					<PageInput
						label="AWS Key Secret"
						help="AWS key secret."
						type="text"
						placeholder="Key secret"
						value={secr.value}
						onChange={(val: string): void => {
							this.set('value', val);
						}}
					/>
				</div>
				<div style={css.group}>
					<PageInfo
						fields={[
							{
								label: 'ID',
								value: this.props.secret.id || 'None',
							},
						]}
					/>
					<PageSelect
						label="Type"
						disabled={this.state.disabled || Constants.user}
						help="Secret provider."
						value={secr.type}
						onChange={(val): void => {
							this.set('type', val);
						}}
					>
						<option value="aws">AWS</option>
					</PageSelect>
					<PageSelect
						disabled={this.state.disabled}
						hidden={Constants.user}
						label="Organization"
						help="Organization for secret. Select node to create a secret for nodes."
						value={secr.organization}
						onChange={(val): void => {
							this.set('organization', val);
						}}
					>
						{organizationsSelect}
					</PageSelect>
				</div>
			</div>
			<PageSave
				style={css.save}
				hidden={!this.state.secret}
				message={this.state.message}
				changed={this.state.changed}
				disabled={this.state.disabled}
				light={true}
				onCancel={(): void => {
					this.setState({
						...this.state,
						changed: false,
						secret: null,
					});
				}}
				onSave={this.onSave}
			/>
		</div>;
	}
}
