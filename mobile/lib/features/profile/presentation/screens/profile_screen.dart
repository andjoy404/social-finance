import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:social_finance/core/models/role.dart';
import 'package:social_finance/core/providers/theme_provider.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/utils/phone_utils.dart';
import 'package:social_finance/core/utils/rw_utils.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/double_back_exit_scope.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';
import 'package:social_finance/features/notification/data/models/notification_models.dart';
import 'package:social_finance/core/widgets/resident_avatar.dart';

class ProfileScreen extends ConsumerStatefulWidget {
  const ProfileScreen({super.key});

  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        ProviderScope.containerOf(
          context,
        ).read(authRepositoryProvider.notifier).reloadProfile().then((_) {
          if (mounted) setState(() {});
        });
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final authState = ref.watch(authRepositoryProvider);
    final authRepo = ref.read(authRepositoryProvider.notifier);

    final user = authState?.valueOrNull;
    final userName = user?['name'] ?? 'Pengguna';
    final role = user?['role'] as AppRole? ?? AppRole.warga;
    final jabatan = (user?['jabatan'] ?? '').toString().trim().toLowerCase();
    final isSpecialOfficer = jabatan == 'keamanan' || jabatan == 'kebersihan';
    final rt = user?['rt'] ?? '';
    final rawRw = (user?['rw'] as String?) ?? '';
    final rwMemberships =
        (user?['rw_memberships'] as List?)
            ?.whereType<Map<String, dynamic>>()
            .toList() ??
        <Map<String, dynamic>>[];
    final resolvedRw = looksLikeUuid(rawRw)
        ? resolveRwNumber(rawRw, rwMemberships)
        : rawRw;
    final rw = resolvedRw ?? '';
    final rtRwDisplay = [
      if (rt.isNotEmpty) 'RT $rt',
      if (rw.isNotEmpty) 'RW $rw',
    ].join(' / ');
    final rawPhone = user?['phone'] as String?;
    final phoneDisplay = PhoneUtils.formatDisplay(rawPhone);
    final emailDisplay = (user?['email'] as String?) ?? '-';
    final rtName = (user?['rt_name'] as String?) ?? '';
    final rtAddress = (user?['rt_address'] as String?) ?? '';
    final regionDisplay = [
      if (rtName.isNotEmpty) rtName,
      if (rtAddress.isNotEmpty) rtAddress,
    ].join(' · ');
    final rawHouseNumber = (user?['house_number'] as String?)?.trim();
    final houseNumberDisplay =
        (rawHouseNumber != null && rawHouseNumber.isNotEmpty)
        ? rawHouseNumber
        : '-';

    final isDark = theme.brightness == Brightness.dark;

    return DoubleBackExitScope(
      child: Scaffold(
        appBar: AppBar(
          automaticallyImplyLeading: false,
          centerTitle: false,
          title: const MenuAppBarTitle(title: 'Profil'),
        ),
        body: RefreshIndicator(
          onRefresh: () async {
            try {
              await ref.read(authRepositoryProvider.notifier).reloadProfile();
              if (mounted) {
                setState(() {});
              }
            } catch (e) {
              if (context.mounted) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(
                    content: Text('Gagal memuat ulang profil: $e'),
                    backgroundColor: AppColors.danger,
                  ),
                );
              }
            }
          },
          child: SingleChildScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.symmetric(horizontal: 20.0, vertical: 12.0),
            child: Column(
            children: [
              // Top right compact Logout button
              Align(
                alignment: Alignment.centerRight,
                child: SizedBox(
                  height: 36,
                  child: OutlinedButton.icon(
                    onPressed: () => _showLogoutDialog(context, authRepo),
                    icon: const Icon(
                      Icons.logout,
                      size: 16,
                      color: AppColors.expense,
                    ),
                    label: const Text(
                      'Keluar',
                      style: TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: AppColors.expense,
                      ),
                    ),
                    style: OutlinedButton.styleFrom(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 6,
                      ),
                      side: BorderSide(
                        color: AppColors.expense.withValues(
                          alpha: isDark ? 0.6 : 0.4,
                        ),
                      ),
                      backgroundColor: isDark
                          ? AppColors.darkDangerSoft
                          : AppColors.lightDangerSoft,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(AppRadius.base),
                      ),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 8),
              // Avatar — same avatar source as menu Warga / pengurus views.
              ResidentAvatar(
                name: userName,
                avatarPath: user?['avatar_path'] as String?,
                size: 80,
              ),
              const SizedBox(height: 16),
              // Identity
              Text(userName, style: theme.textTheme.headlineSmall),
              const SizedBox(height: 8),
              AppBadge(
                label: appRoleDisplayName(role),
                isNeonStyle: isDark,
                backgroundColor: isDark
                    ? null
                    : theme.colorScheme.primary.withValues(alpha: 0.12),
              ),
              const SizedBox(height: 4),
              Text(
                rtRwDisplay.isNotEmpty ? rtRwDisplay : 'RT / RW',
                style: theme.textTheme.bodySmall,
              ),
              const SizedBox(height: 24),

              // Profile items
              _buildProfileSection(theme, [
                _ProfileItemData(
                  icon: Icons.home,
                  title: 'RT / RW',
                  subtitle: rtRwDisplay.isNotEmpty ? rtRwDisplay : '-',
                ),
                _ProfileItemData(
                  icon: Icons.location_on_outlined,
                  title: 'Nomor Rumah',
                  subtitle: houseNumberDisplay,
                ),
                _ProfileItemData(
                  icon: Icons.phone_android,
                  title: 'Telepon',
                  subtitle: phoneDisplay,
                  onTap: isSpecialOfficer
                      ? null
                      : () => _showEditPhoneDialog(
                          context,
                          rawPhone ?? '',
                          userName,
                          emailDisplay == '-' ? null : emailDisplay,
                        ),
                ),
                _ProfileItemData(
                  icon: Icons.email_outlined,
                  title: 'Email',
                  subtitle: emailDisplay,
                  onTap: isSpecialOfficer
                      ? null
                      : () => _showEditEmailDialog(
                          context,
                          emailDisplay == '-' ? '' : emailDisplay,
                          userName,
                          rawPhone,
                        ),
                ),
                if (isSpecialOfficer) ...[
                  _ProfileItemData(
                    icon: Icons.badge_outlined,
                    title: 'Jabatan',
                    subtitle: jabatan == 'keamanan'
                        ? 'Petugas Keamanan'
                        : 'Petugas Kebersihan',
                  ),
                  _ProfileItemData(
                    icon: Icons.shield_outlined,
                    title: 'Role',
                    subtitle: appRoleDisplayName(role),
                  ),
                  _ProfileItemData(
                    icon: Icons.location_city_outlined,
                    title: 'Wilayah',
                    subtitle: regionDisplay.isNotEmpty
                        ? regionDisplay
                        : (rtRwDisplay.isNotEmpty ? rtRwDisplay : '-'),
                  ),
                ],
                if (!isSpecialOfficer && canUserAccessNotificationMenu(user))
                  _ProfileItemData(
                    icon: Icons.notifications_active_outlined,
                    title: 'Kelola Notifikasi',
                    subtitle: 'Template & Pengaturan Push Notifikasi',
                    onTap: () => context.push('/home/notifikasi'),
                  ),
                _ProfileItemData(
                  icon: Icons.security,
                  title: 'Keamanan',
                  subtitle: 'Ubah kata sandi',
                  onTap: () => _showChangePasswordDialog(context),
                ),
                _ProfileItemData(
                  icon: Icons.palette_outlined,
                  title: 'Tema Tampilan',
                  subtitle: switch (ref.watch(themeProvider)) {
                    AppThemeMode.light => 'Terang',
                    AppThemeMode.dark => 'Gelap',
                    AppThemeMode.system => 'Ikuti Sistem',
                  },
                  onTap: () => _showThemeSelectionDialog(context, ref),
                ),
                const _ProfileItemData(
                  icon: Icons.info_outline,
                  title: 'Tentang',
                  subtitle: 'Social Finance v1.0.0',
                ),
              ], isDark),
              const SizedBox(height: 24),
            ],
          ),
        ),
      ),
    ),
  );
  }

  Widget _buildProfileSection(
    ThemeData theme,
    List<_ProfileItemData> items,
    bool isDark,
  ) {
    return Column(
      children: items.map((item) {
        return Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: AppCard(
            padding: EdgeInsets.zero,
            child: InkWell(
              onTap: item.onTap,
              borderRadius: BorderRadius.circular(AppRadius.base),
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 14,
                ),
                child: Row(
                  children: [
                    Icon(item.icon, size: 20, color: theme.colorScheme.primary),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(item.title, style: theme.textTheme.titleMedium),
                          Text(item.subtitle, style: theme.textTheme.bodySmall),
                        ],
                      ),
                    ),
                    if (item.onTap != null)
                      Icon(
                        Icons.chevron_right,
                        size: 20,
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                  ],
                ),
              ),
            ),
          ),
        );
      }).toList(),
    );
  }

  void _showChangePasswordDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (dialogContext) => _ChangePasswordDialog(
        authRepo: ref.read(authRepositoryProvider.notifier),
      ),
    );
  }

  void _showEditPhoneDialog(
    BuildContext context,
    String currentPhone,
    String userName,
    String? currentEmail,
  ) {
    showDialog(
      context: context,
      builder: (dialogContext) => _EditPhoneDialog(
        authRepo: ref.read(authRepositoryProvider.notifier),
        currentPhone: currentPhone,
        userName: userName,
        currentEmail: currentEmail,
      ),
    );
  }

  void _showEditEmailDialog(
    BuildContext context,
    String currentEmail,
    String userName,
    String? currentPhone,
  ) {
    showDialog(
      context: context,
      builder: (dialogContext) => _EditEmailDialog(
        authRepo: ref.read(authRepositoryProvider.notifier),
        currentEmail: currentEmail,
        userName: userName,
        currentPhone: currentPhone,
      ),
    );
  }

  void _showLogoutDialog(BuildContext context, dynamic authRepo) {
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Konfirmasi Keluar'),
        content: const Text('Apakah Anda yakin ingin keluar dari aplikasi?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('Batal'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(backgroundColor: AppColors.expense),
            onPressed: () async {
              Navigator.of(dialogContext).pop();
              await authRepo.logout();
              if (context.mounted) {
                context.go('/login');
              }
            },
            child: const Text('Keluar'),
          ),
        ],
      ),
    );
  }

  void _showThemeSelectionDialog(BuildContext context, WidgetRef ref) {
    final currentMode = ref.read(themeProvider);
    showDialog(
      context: context,
      builder: (dialogContext) => SimpleDialog(
        title: const Text('Pilih Tema'),
        children: [
          _themeRadioTile(
            dialogContext,
            ref,
            'Ikuti Sistem',
            AppThemeMode.system,
            currentMode,
          ),
          _themeRadioTile(
            dialogContext,
            ref,
            'Terang',
            AppThemeMode.light,
            currentMode,
          ),
          _themeRadioTile(
            dialogContext,
            ref,
            'Gelap',
            AppThemeMode.dark,
            currentMode,
          ),
        ],
      ),
    );
  }

  Widget _themeRadioTile(
    BuildContext context,
    WidgetRef ref,
    String label,
    AppThemeMode value,
    AppThemeMode currentMode,
  ) {
    final isSelected = currentMode == value;
    return SimpleDialogOption(
      onPressed: () {
        ref.read(themeProvider.notifier).setMode(value);
        Navigator.of(context).pop();
      },
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Row(
          children: [
            Icon(
              isSelected ? Icons.radio_button_checked : Icons.radio_button_off,
              color: isSelected
                  ? Theme.of(context).colorScheme.primary
                  : Theme.of(context).colorScheme.onSurfaceVariant,
              size: 20,
            ),
            const SizedBox(width: 12),
            Text(label, style: const TextStyle(fontSize: 15)),
          ],
        ),
      ),
    );
  }
}

class _ProfileItemData {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback? onTap;

  const _ProfileItemData({
    required this.icon,
    required this.title,
    required this.subtitle,
    this.onTap,
  });
}

class _ChangePasswordDialog extends StatefulWidget {
  final AuthRepository authRepo;

  const _ChangePasswordDialog({required this.authRepo});

  @override
  State<_ChangePasswordDialog> createState() => _ChangePasswordDialogState();
}

class _ChangePasswordDialogState extends State<_ChangePasswordDialog> {
  final _currentController = TextEditingController();
  final _newController = TextEditingController();
  final _confirmController = TextEditingController();

  bool _obscureCurrent = true;
  bool _obscureNew = true;
  bool _obscureConfirm = true;
  bool _loading = false;
  String? _errorMessage;

  @override
  void dispose() {
    _currentController.dispose();
    _newController.dispose();
    _confirmController.dispose();
    super.dispose();
  }

  Future<void> _handleSubmit() async {
    final current = _currentController.text;
    final newPass = _newController.text;
    final confirm = _confirmController.text;

    if (current.isEmpty) {
      setState(() => _errorMessage = 'Kata sandi saat ini harus diisi.');
      return;
    }
    if (newPass.isEmpty) {
      setState(() => _errorMessage = 'Kata sandi baru harus diisi.');
      return;
    }
    if (newPass.length < 8) {
      setState(() => _errorMessage = 'Kata sandi baru minimal 8 karakter.');
      return;
    }
    if (newPass != confirm) {
      setState(() => _errorMessage = 'Konfirmasi kata sandi tidak cocok.');
      return;
    }

    setState(() {
      _loading = true;
      _errorMessage = null;
    });

    try {
      final authRepo = widget.authRepo;
      await authRepo.changePassword(
        currentPassword: current,
        newPassword: newPass,
      );

      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Kata sandi berhasil diubah.'),
            backgroundColor: AppColors.success,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _errorMessage = e.toString().replaceFirst('Exception: ', '');
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return AlertDialog(
      title: const Text('Ubah Kata Sandi'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (_errorMessage != null) ...[
              Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: AppColors.danger.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                  border: Border.all(
                    color: AppColors.danger.withValues(alpha: 0.3),
                  ),
                ),
                child: Text(
                  _errorMessage!,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: AppColors.danger,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              const SizedBox(height: 14),
            ],
            TextField(
              controller: _currentController,
              obscureText: _obscureCurrent,
              decoration: InputDecoration(
                labelText: 'Kata Sandi Saat Ini',
                suffixIcon: IconButton(
                  icon: Icon(
                    _obscureCurrent ? Icons.visibility_off : Icons.visibility,
                    size: 20,
                  ),
                  onPressed: () =>
                      setState(() => _obscureCurrent = !_obscureCurrent),
                ),
              ),
              enabled: !_loading,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _newController,
              obscureText: _obscureNew,
              decoration: InputDecoration(
                labelText: 'Kata Sandi Baru (min. 8 karakter)',
                suffixIcon: IconButton(
                  icon: Icon(
                    _obscureNew ? Icons.visibility_off : Icons.visibility,
                    size: 20,
                  ),
                  onPressed: () => setState(() => _obscureNew = !_obscureNew),
                ),
              ),
              enabled: !_loading,
            ),
            const SizedBox(height: 12),
            TextField(
              controller: _confirmController,
              obscureText: _obscureConfirm,
              decoration: InputDecoration(
                labelText: 'Konfirmasi Kata Sandi Baru',
                suffixIcon: IconButton(
                  icon: Icon(
                    _obscureConfirm ? Icons.visibility_off : Icons.visibility,
                    size: 20,
                  ),
                  onPressed: () =>
                      setState(() => _obscureConfirm = !_obscureConfirm),
                ),
              ),
              enabled: !_loading,
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _loading ? null : () => Navigator.of(context).pop(),
          child: const Text('Batal'),
        ),
        FilledButton(
          onPressed: _loading ? null : _handleSubmit,
          child: _loading
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
              : const Text('Simpan'),
        ),
      ],
    );
  }
}

class _EditPhoneDialog extends StatefulWidget {
  final AuthRepository authRepo;
  final String currentPhone;
  final String userName;
  final String? currentEmail;

  const _EditPhoneDialog({
    required this.authRepo,
    required this.currentPhone,
    required this.userName,
    this.currentEmail,
  });

  @override
  State<_EditPhoneDialog> createState() => _EditPhoneDialogState();
}

class _EditPhoneDialogState extends State<_EditPhoneDialog> {
  late final TextEditingController _phoneController;
  bool _loading = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    _phoneController = TextEditingController(text: widget.currentPhone);
  }

  @override
  void dispose() {
    _phoneController.dispose();
    super.dispose();
  }

  Future<void> _handleSubmit() async {
    final raw = _phoneController.text.trim();
    final validationError = PhoneUtils.validatePhone(raw);
    if (validationError != null) {
      setState(() => _errorMessage = validationError);
      return;
    }

    final normalized = PhoneUtils.normalizePhone(raw);

    setState(() {
      _loading = true;
      _errorMessage = null;
    });

    try {
      await widget.authRepo.updateProfile(
        fullName: widget.userName,
        phone: normalized,
        email: widget.currentEmail,
      );

      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Nomor telepon berhasil diperbarui.'),
            backgroundColor: AppColors.success,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _errorMessage = e
              .toString()
              .replaceFirst('Exception: ', '')
              .replaceFirst('AuthError: ', '');
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return AlertDialog(
      title: const Text('Ubah Nomor Telepon'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (_errorMessage != null) ...[
              Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: AppColors.danger.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                  border: Border.all(
                    color: AppColors.danger.withValues(alpha: 0.3),
                  ),
                ),
                child: Text(
                  _errorMessage!,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: AppColors.danger,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              const SizedBox(height: 14),
            ],
            TextField(
              controller: _phoneController,
              keyboardType: TextInputType.phone,
              decoration: const InputDecoration(
                labelText: 'Nomor Telepon',
                hintText: 'Contoh: 081298918220',
              ),
              enabled: !_loading,
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _loading ? null : () => Navigator.of(context).pop(),
          child: const Text('Batal'),
        ),
        FilledButton(
          onPressed: _loading ? null : _handleSubmit,
          child: _loading
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
              : const Text('Simpan'),
        ),
      ],
    );
  }
}

class _EditEmailDialog extends StatefulWidget {
  final AuthRepository authRepo;
  final String currentEmail;
  final String userName;
  final String? currentPhone;

  const _EditEmailDialog({
    required this.authRepo,
    required this.currentEmail,
    required this.userName,
    this.currentPhone,
  });

  @override
  State<_EditEmailDialog> createState() => _EditEmailDialogState();
}

class _EditEmailDialogState extends State<_EditEmailDialog> {
  late final TextEditingController _emailController;
  bool _loading = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    _emailController = TextEditingController(text: widget.currentEmail);
  }

  @override
  void dispose() {
    _emailController.dispose();
    super.dispose();
  }

  Future<void> _handleSubmit() async {
    final email = _emailController.text.trim();
    if (email.isEmpty) {
      setState(() => _errorMessage = 'Email wajib diisi.');
      return;
    }
    final emailRegex = RegExp(r'^[^@\s]+@[^@\s]+\.[^@\s]+$');
    if (!emailRegex.hasMatch(email)) {
      setState(() => _errorMessage = 'Format email tidak valid.');
      return;
    }

    setState(() {
      _loading = true;
      _errorMessage = null;
    });

    try {
      await widget.authRepo.updateProfile(
        fullName: widget.userName,
        phone: widget.currentPhone,
        email: email,
      );

      if (mounted) {
        Navigator.of(context).pop();
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Email berhasil diperbarui.'),
            backgroundColor: AppColors.success,
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _errorMessage = e
              .toString()
              .replaceFirst('Exception: ', '')
              .replaceFirst('AuthError: ', '');
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return AlertDialog(
      title: const Text('Ubah Email'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (_errorMessage != null) ...[
              Container(
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: AppColors.danger.withValues(alpha: 0.12),
                  borderRadius: BorderRadius.circular(AppRadius.sm),
                  border: Border.all(
                    color: AppColors.danger.withValues(alpha: 0.3),
                  ),
                ),
                child: Text(
                  _errorMessage!,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: AppColors.danger,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              const SizedBox(height: 14),
            ],
            TextField(
              controller: _emailController,
              keyboardType: TextInputType.emailAddress,
              decoration: const InputDecoration(
                labelText: 'Email',
                hintText: 'nama@domain.com',
              ),
              enabled: !_loading,
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _loading ? null : () => Navigator.of(context).pop(),
          child: const Text('Batal'),
        ),
        FilledButton(
          onPressed: _loading ? null : _handleSubmit,
          child: _loading
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
              : const Text('Simpan'),
        ),
      ],
    );
  }
}
