package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/Binmave/binmave-cli/internal/auth"
	"github.com/Binmave/binmave-cli/internal/config"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Binmave server",
	Long: `Authenticate with the Binmave server using your browser.

This will open your default browser to complete the authentication flow.
After successful authentication, your credentials will be stored locally.`,
	RunE: runLogin,
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Check if already logged in
	token, err := auth.LoadToken()
	if err != nil {
		return fmt.Errorf("failed to check existing credentials: %w", err)
	}

	if token != nil && token.IsValid() {
		fmt.Println("You are already logged in.")
		fmt.Println("Use 'binmave logout' to log out first, or 'binmave whoami' to see your current user.")
		return nil
	}

	fmt.Printf("Authenticating with %s...\n\n", config.GetServer())

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nAuthentication cancelled.")
		cancel()
	}()

	// Perform login
	token, err = auth.Login(ctx)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Get user info
	userInfo, err := auth.GetUserInfo(token)
	if err != nil {
		// Login succeeded but couldn't get user info - still successful
		fmt.Println("\n✓ Login successful!")
		return nil
	}

	fmt.Printf("\n✓ Login successful! Welcome, %s\n", userInfo.GetDisplayName())
	if userInfo.Email != "" {
		fmt.Printf("  Email: %s\n", userInfo.Email)
	}
	if userInfo.Role() != "" {
		fmt.Printf("  Role: %s\n", userInfo.Role())
	}

	return nil
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and clear stored credentials",
	Long:  `Log out from the Binmave server and remove locally stored credentials.`,
	RunE:  runLogout,
}

func runLogout(cmd *cobra.Command, args []string) error {
	token, err := auth.LoadToken()
	if err != nil {
		return fmt.Errorf("failed to check credentials: %w", err)
	}

	if token == nil {
		fmt.Println("You are not logged in.")
		return nil
	}

	if err := auth.DeleteToken(); err != nil {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}

	fmt.Println("✓ Logged out successfully.")
	return nil
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show information about the current user",
	Long:  `Display information about the currently authenticated user.`,
	RunE:  runWhoami,
}

func runWhoami(cmd *cobra.Command, args []string) error {
	token, err := auth.GetValidToken()
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	if token == nil {
		fmt.Println("You are not logged in. Run 'binmave login' to authenticate.")
		return nil
	}

	userInfo, err := auth.GetUserInfo(token)
	if err != nil {
		return fmt.Errorf("failed to get user info: %w", err)
	}

	if IsJSONOutput() {
		return printJSON(userInfo)
	}

	fmt.Printf("User: %s\n", userInfo.GetDisplayName())
	if userInfo.Email != "" {
		fmt.Printf("Email: %s\n", userInfo.Email)
	}
	if userInfo.Role() != "" {
		fmt.Printf("Role: %s\n", userInfo.Role())
	}
	fmt.Printf("Server: %s\n", config.GetServer())
	fmt.Printf("Token expires: %s\n", token.ExpiresAt.Format("2006-01-02 15:04:05"))

	return nil
}
